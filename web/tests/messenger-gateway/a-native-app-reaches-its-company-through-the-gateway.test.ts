import { afterAll, beforeAll, expect, test } from 'bun:test';
import { SQL } from 'bun';
import { mkdtempSync, openSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { AbstractRelay } from 'nostr-tools/abstract-relay';
import { finalizeEvent, generateSecretKey, getPublicKey, verifyEvent, type EventTemplate } from 'nostr-tools/pure';
import { controlPlane, issueAgentKey, provisionCompany, sessionForHost } from '../../src/lib/server/control-plane';
import { connectToGateway, type GatewayConnection } from '../../../host/relay/gateway-socket';
import type { Dispatch } from '../../../host/relay/forward';
import { MessengerRelay } from '../../../host/relay/messenger-calls';
import { messengerStoreOf } from '../../../host/relay/messenger-store';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../integration/supabase-environment';

const repositoryRoot = resolve(import.meta.dir, '../../..');
const gatewayDirectory = join(repositoryRoot, 'workers/connection-gateway');
const startupMilliseconds = 120_000;
const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };
const client = controlPlane(credentials);
const stamp = Date.now();
const slug = `gateway-app-${stamp}`;
const publicHost = `${slug}.example.test`;
const relayDatabase = `buzz_gateway_app_${stamp}`;
const mediaBucket = `buzz-gateway-app-${stamp}`;
const workDirectory = mkdtempSync(join(tmpdir(), 'messenger-gateway-'));
const started: Bun.Subprocess[] = [];

let companyID = '';
let hostAccountID = '';
let gatewayPort = 0;
let relayPort = 0;
let host: GatewayConnection | null = null;

function databaseURL(): string {
	const named = process.env.SUPABASE_DB_URL ?? '';
	if (!named) throw new Error('SUPABASE_DB_URL is not set; run this as "bun run test:messenger-gateway"');
	return named;
}

function localPlaneSetting(name: string): string {
	const status = Bun.spawnSync(['supabase', 'status', '--env', '--output-format', 'text'], { cwd: repositoryRoot });
	const line = status.stdout
		.toString()
		.split('\n')
		.find((entry) => entry.startsWith(`${name}=`));
	const value = line?.slice(name.length + 1).replace(/^(["'])(.*)\1$/, '$2') ?? '';
	if (!value) throw new Error(`the local plane names no ${name}`);
	return value;
}

function relayBinary(): string {
	const directory = process.platform === 'darwin' ? 'buzz-relay-darwin-arm64' : 'buzz-relay';
	return join(repositoryRoot, '.dependency', directory, 'buzz-relay');
}

function freePort(): number {
	const probe = Bun.serve({ port: 0, fetch: () => new Response('') });
	const port = probe.port;
	probe.stop(true);
	if (port === undefined) throw new Error('the port probe bound no port');
	return port;
}

function spawnLogged(name: string, command: string[], options: { cwd?: string; env?: Record<string, string> } = {}): Bun.Subprocess {
	const log = openSync(join(workDirectory, `${name}.log`), 'a');
	const child = Bun.spawn(command, {
		cwd: options.cwd,
		env: { ...process.env, ...options.env },
		stdout: log,
		stderr: log
	});
	started.push(child);
	return child;
}

async function untilAnswered(what: string, probe: () => Promise<boolean>, child: Bun.Subprocess): Promise<void> {
	for (let attempt = 0; attempt < 240; attempt += 1) {
		if (await probe().catch(() => false)) return;
		if (child.exitCode !== null) throw new Error(`${what} exited with ${child.exitCode}; see ${workDirectory}/${what}.log`);
		await Bun.sleep(250);
	}
	throw new Error(`${what} never answered; see ${workDirectory}/${what}.log`);
}

async function startTheMessengerRelay(): Promise<void> {
	const administrator = new SQL(databaseURL());
	await administrator.unsafe(`create database ${relayDatabase}`);
	await administrator.close();
	const relayDatabaseURL = new URL(databaseURL());
	relayDatabaseURL.pathname = `/${relayDatabase}`;

	const redisPort = freePort();
	const redis = spawnLogged('redis', ['redis-server', '--port', String(redisPort), '--save', '', '--appendonly', 'no']);
	await untilAnswered('redis', async () => (await Bun.connect({ hostname: '127.0.0.1', port: redisPort, socket: { data() {} } })) !== null, redis);

	const bucket = await client.storage.createBucket(mediaBucket, { public: false });
	if (bucket.error) throw new Error(`the media bucket could not be made: ${bucket.error.message}`);

	relayPort = freePort();
	const relay = spawnLogged('buzz-relay', [relayBinary()], {
		env: {
			BUZZ_S3_ENDPOINT: localPlaneSetting('STORAGE_S3_URL'),
			BUZZ_S3_ACCESS_KEY: localPlaneSetting('S3_PROTOCOL_ACCESS_KEY_ID'),
			BUZZ_S3_SECRET_KEY: localPlaneSetting('S3_PROTOCOL_ACCESS_KEY_SECRET'),
			BUZZ_S3_REGION: localPlaneSetting('S3_PROTOCOL_REGION'),
			BUZZ_S3_BUCKET: mediaBucket,
			BUZZ_S3_ADDRESSING_STYLE: 'path',
			BUZZ_MEDIA_BASE_URL: `https://${publicHost}/media`,
			DATABASE_URL: relayDatabaseURL.toString(),
			REDIS_URL: `redis://127.0.0.1:${redisPort}`,
			BUZZ_BIND_ADDR: `127.0.0.1:${relayPort}`,
			BUZZ_HEALTH_PORT: String(freePort()),
			RELAY_URL: `wss://${publicHost}`,
			BUZZ_AUTO_MIGRATE: '1',
			BUZZ_REQUIRE_RELAY_MEMBERSHIP: 'false',
			BUZZ_GIT_CONFORMANCE_PROBE: 'false',
			BUZZ_GIT_REPO_PATH: join(workDirectory, 'repos'),
			BUZZ_RELAY_PRIVATE_KEY: Buffer.from(generateSecretKey()).toString('hex'),
			RUST_LOG: 'buzz_relay=info,buzz_media=info'
		}
	});
	await untilAnswered(
		'buzz-relay',
		async () => (await fetch(`http://127.0.0.1:${relayPort}/info`, { headers: { Host: publicHost } })).ok,
		relay
	);
}

async function startTheGateway(): Promise<void> {
	gatewayPort = freePort();
	const configuration = {
		...JSON.parse(await Bun.file(join(gatewayDirectory, 'wrangler.jsonc')).text()),
		main: join(gatewayDirectory, 'src/index.ts'),
		vars: { SUPABASE_URL: projectURL, SUPABASE_PUBLISHABLE_KEY: publishableKey }
	};
	const configurationPath = join(workDirectory, 'gateway.jsonc');
	writeFileSync(configurationPath, JSON.stringify(configuration));
	const gateway = spawnLogged(
		'gateway',
		[
			'bunx', 'wrangler', 'dev', '--local', '--port', String(gatewayPort), '--inspector-port', '0',
			'--persist-to', join(workDirectory, 'gateway-state'), '--config', configurationPath
		],
		{ cwd: join(repositoryRoot, 'web') }
	);
	await untilAnswered('gateway', async () => (await fetch(`http://127.0.0.1:${gatewayPort}/missing`)).status === 404, gateway);
}

async function connectTheHost(): Promise<void> {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Gateway App Company', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	const { apiKey } = await issueAgentKey(client, companyID, 'host');
	const session = await sessionForHost(credentials, apiKey);
	const hostAccess = { projectURL, apiKey: publishableKey, accessToken: async () => session.accessToken };
	const messenger = new MessengerRelay(`http://127.0.0.1:${relayPort}`, messengerStoreOf(companyID, hostAccess));
	const dispatch = { serveMessenger: (capability: string, body: Record<string, unknown>) => messenger.serve(capability, body) };
	host = connectToGateway({
		gatewayURL: `ws://127.0.0.1:${gatewayPort}`,
		companyID,
		hostAccessToken: async () => session.accessToken,
		dispatch: dispatch as unknown as Dispatch,
		byteCeiling: 3_000_000,
		messengerRelayURL: `ws://127.0.0.1:${relayPort}`,
		report: (line) => writeFileSync(join(workDirectory, 'host.log'), `${line}\n`, { flag: 'a' })
	});
	const users = await client.auth.admin.listUsers({ perPage: 1000 });
	hostAccountID = users.data.users.find((user) => user.email?.startsWith(`host.${companyID}@`))?.id ?? '';
}

function dialledThroughTheGateway(port: () => number): typeof WebSocket {
	return class extends WebSocket {
		constructor(address: string | URL) {
			const dialled = new URL(address);
			const through = `ws://127.0.0.1:${port()}${dialled.pathname}${dialled.search}`;
			super(through, { headers: { Host: dialled.host } } as unknown as string[]);
		}
	};
}

async function appConnectedAs(secretKey: Uint8Array, port: () => number): Promise<AbstractRelay> {
	const relay = new AbstractRelay(`wss://${publicHost}`, { verifyEvent, websocketImplementation: dialledThroughTheGateway(port) });
	const sign = async (template: EventTemplate) => finalizeEvent(template, secretKey);
	const challenged = Promise.withResolvers<void>();
	relay.onauth = async (template: EventTemplate) => {
		challenged.resolve();
		return sign(template);
	};
	await relay.connect();
	await challenged.promise;
	await relay.auth(sign);
	return relay;
}

beforeAll(async () => {
	await startTheMessengerRelay();
	await startTheGateway();
	await connectTheHost();
	for (let attempt = 0; attempt < 100; attempt += 1) {
		const probe = await fetch(`http://127.0.0.1:${gatewayPort}/info`, { headers: { Host: publicHost } });
		if (probe.ok) return;
		await Bun.sleep(100);
	}
	throw new Error(`the host never connected to the gateway; see ${workDirectory}`);
}, startupMilliseconds);

afterAll(async () => {
	host?.close();
	for (const child of started.reverse()) {
		child.kill('SIGTERM');
		await Promise.race([child.exited, Bun.sleep(3000)]);
		child.kill('SIGKILL');
	}
	if (companyID) {
		const kept = await client.storage.from('asset').list(`${companyID}/shared/transfer`);
		const keptPaths = (kept.data ?? []).map((object) => `${companyID}/shared/transfer/${object.name}`);
		if (keptPaths.length > 0) await client.storage.from('asset').remove(keptPaths);
		await client.from('company').delete().eq('id', companyID);
	}
	await client.storage.emptyBucket(mediaBucket);
	await client.storage.deleteBucket(mediaBucket);
	if (hostAccountID) await client.auth.admin.deleteUser(hostAccountID);
	const administrator = new SQL(databaseURL());
	await administrator.unsafe(`drop database if exists ${relayDatabase} with (force)`);
	await administrator.close();
}, startupMilliseconds);

test('the relay information document answers through the gateway, from the relay on the host', async () => {
	const answered = await fetch(`http://127.0.0.1:${gatewayPort}/`, {
		headers: { Host: publicHost, Accept: 'application/nostr+json' }
	});
	expect(answered.status).toBe(200);
	const document = (await answered.json()) as { name?: unknown };
	expect(typeof document.name).toBe('string');
});

test('an app authenticates with NIP-42, publishes, and another app receives it, all through the gateway', async () => {
	const sender = generateSecretKey();
	const reader = generateSecretKey();
	const sending = await appConnectedAs(sender, () => gatewayPort);
	const reading = await appConnectedAs(reader, () => gatewayPort);

	const note = finalizeEvent({ kind: 1, created_at: Math.floor(Date.now() / 1000), tags: [], content: `through the gateway ${stamp}` }, sender);
	const received = new Promise<string>((settle, refuse) => {
		const timer = setTimeout(() => refuse(new Error('the reader never received the note')), 10_000);
		reading.subscribe([{ ids: [note.id] }], {
			onevent: (event) => {
				clearTimeout(timer);
				settle(event.content);
			}
		});
	});
	await Bun.sleep(200);
	await sending.publish(note);
	expect(await received).toBe(`through the gateway ${stamp}`);
	expect(getPublicKey(sender)).toBe(note.pubkey);
	sending.close();
	reading.close();
}, 30_000);

function rawAppSocket(): WebSocket {
	const ThroughTheGateway = dialledThroughTheGateway(() => gatewayPort);
	const socket = new ThroughTheGateway(`wss://${publicHost}/`);
	return socket;
}

function closeCodeOf(socket: WebSocket): Promise<number> {
	return new Promise((settle) => socket.addEventListener('close', (closed) => settle(closed.code)));
}

function opened(socket: WebSocket): Promise<void> {
	return new Promise((settle) => socket.addEventListener('open', () => settle()));
}

test('a frame over 512 KiB is refused by the gateway, which closes the app as too big', async () => {
	const socket = rawAppSocket();
	const closed = closeCodeOf(socket);
	await opened(socket);
	socket.send(JSON.stringify(['EVENT', { content: 'x'.repeat(512 * 1024) }]));
	expect(await closed).toBe(1009);
}, 30_000);

test('the added round trip of a REQ through the gateway, against the same REQ on loopback', async () => {
	const secretKey = generateSecretKey();
	const throughTheGateway = await appConnectedAs(secretKey, () => gatewayPort);
	const onLoopback = await appConnectedAs(secretKey, () => relayPort);

	const gatewayMilliseconds = await medianRoundTrip(throughTheGateway);
	const loopbackMilliseconds = await medianRoundTrip(onLoopback);
	console.log(
		`REQ→EOSE median over 40: ${gatewayMilliseconds.toFixed(2)} ms through the gateway, ` +
			`${loopbackMilliseconds.toFixed(2)} ms on loopback, ${(gatewayMilliseconds - loopbackMilliseconds).toFixed(2)} ms added`
	);
	expect(gatewayMilliseconds).toBeGreaterThan(0);
	throughTheGateway.close();
	onLoopback.close();
}, 60_000);

const onePixelPNG = Uint8Array.from(
	atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='),
	(character) => character.charCodeAt(0)
);

function blossomAuthorization(secretKey: Uint8Array, tags: string[][], content: string): string {
	const nowSeconds = Math.floor(Date.now() / 1000);
	const event = finalizeEvent(
		{ kind: 24242, created_at: nowSeconds, content, tags: [...tags, ['expiration', String(nowSeconds + 300)]] },
		secretKey
	);
	return `Nostr ${btoa(JSON.stringify(event))}`;
}

test('an app uploads a picture and reads it back through the transfer store, with the range it asks for', async () => {
	const secretKey = generateSecretKey();
	const digest = new Bun.CryptoHasher('sha256').update(onePixelPNG).digest('hex');
	const uploaded = await fetch(`http://127.0.0.1:${gatewayPort}/upload`, {
		method: 'PUT',
		headers: {
			Host: publicHost,
			Authorization: blossomAuthorization(secretKey, [['t', 'upload'], ['x', digest]], 'upload'),
			'Content-Type': 'image/png',
			'X-SHA-256': digest
		},
		body: onePixelPNG
	});
	const uploadAnswer = await uploaded.text();
	expect(uploaded.status, uploadAnswer).toBe(200);
	const descriptor = JSON.parse(uploadAnswer) as { url: string; sha256: string };
	expect(descriptor.url.startsWith(`https://${publicHost}/media/`)).toBe(true);

	const blobPath = new URL(descriptor.url).pathname;
	const readToken = blossomAuthorization(secretKey, [['t', 'get'], ['server', `https://${publicHost}`]], 'get');
	const whole = await fetch(`http://127.0.0.1:${gatewayPort}${blobPath}`, {
		headers: { Host: publicHost, Authorization: readToken }
	});
	expect(whole.status).toBe(200);
	expect(whole.headers.get('content-type')).toBe('image/png');
	const wholeBytes = new Uint8Array(await whole.arrayBuffer());
	expect(new Bun.CryptoHasher('sha256').update(wholeBytes).digest('hex')).toBe(descriptor.sha256);

	const part = await fetch(`http://127.0.0.1:${gatewayPort}${blobPath}`, {
		headers: { Host: publicHost, Authorization: readToken, Range: 'bytes=0-7' }
	});
	expect(part.status).toBe(206);
	expect([...new Uint8Array(await part.arrayBuffer())]).toEqual([...wholeBytes.slice(0, 8)]);

	const refused = await fetch(`http://127.0.0.1:${gatewayPort}${blobPath}`, { headers: { Host: publicHost } });
	expect(refused.status).toBeGreaterThanOrEqual(400);
}, 60_000);

test('a file larger than two transfer chunks and any answer the host may send moves both ways intact', async () => {
	const secretKey = generateSecretKey();
	const file = Uint8Array.from({ length: 13 * 1024 * 1024 + 5 }, (_, index) => (index * 31 + 7) % 251);
	const digest = new Bun.CryptoHasher('sha256').update(file).digest('hex');
	const uploaded = await fetch(`http://127.0.0.1:${gatewayPort}/upload`, {
		method: 'PUT',
		headers: {
			Host: publicHost,
			Authorization: blossomAuthorization(secretKey, [['t', 'upload'], ['x', digest]], 'upload'),
			'Content-Type': 'application/octet-stream',
			'X-SHA-256': digest
		},
		body: file
	});
	const uploadAnswer = await uploaded.text();
	expect(uploaded.status, uploadAnswer).toBe(200);
	const descriptor = JSON.parse(uploadAnswer) as { url: string; sha256: string };
	expect(descriptor.sha256).toBe(digest);

	const readToken = blossomAuthorization(secretKey, [['t', 'get'], ['server', `https://${publicHost}`]], 'get');
	const read = await fetch(`http://127.0.0.1:${gatewayPort}${new URL(descriptor.url).pathname}`, {
		headers: { Host: publicHost, Authorization: readToken }
	});
	expect(read.status).toBe(200);
	const readBytes = new Uint8Array(await read.arrayBuffer());
	expect(readBytes.byteLength).toBe(file.byteLength);
	expect(new Bun.CryptoHasher('sha256').update(readBytes).digest('hex')).toBe(digest);
}, 120_000);

async function medianRoundTrip(relay: AbstractRelay): Promise<number> {
	const samples: number[] = [];
	for (let round = 0; round < 40; round += 1) {
		const began = performance.now();
		await new Promise<void>((settle) => {
			const subscription = relay.subscribe([{ ids: ['0'.repeat(64)] }], {
				oneose: () => {
					subscription.close();
					settle();
				}
			});
		});
		samples.push(performance.now() - began);
	}
	samples.sort((first, second) => first - second);
	return samples[Math.floor(samples.length / 2)] ?? 0;
}

test('when the company computer drops, the app is closed as a restart and may dial again', async () => {
	const socket = rawAppSocket();
	const closed = closeCodeOf(socket);
	await opened(socket);
	host?.close();
	host = null;
	expect(await closed).toBe(1012);
	const refused = await fetch(`http://127.0.0.1:${gatewayPort}/info`, { headers: { Host: publicHost } });
	expect(refused.status).toBe(503);
}, 30_000);

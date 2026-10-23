import { afterAll, beforeAll, expect, test } from 'bun:test';
import { mkdtempSync, openSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { exportJWK, generateKeyPair } from 'jose';
import { x25519 } from '@noble/curves/ed25519.js';
import { base64URLOf } from '../../src/lib/company/seal-to-box';
import { announceBox, boxSessionFor, claimBox, makeBoxServe, verifyBoxCode } from '../../src/lib/server/box';
import { controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

const repositoryRoot = resolve(import.meta.dir, '../../..');
const gatewayDirectory = join(repositoryRoot, 'workers/connection-gateway');
const startupMilliseconds = 120_000;
const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };
const client = controlPlane(credentials);
const stamp = Date.now();
const workDirectory = mkdtempSync(join(tmpdir(), 'boxes-share-work-'));
const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey,
	GATEWAY_URL: 'https://gateway.example.test'
};
const appURL = 'https://intern.example.test';

let gateway: Bun.Subprocess | null = null;
let gatewayPort = 0;
let companyID = '';
let headOffice = '';
let annex = '';

type Frame = { kind: string; requestID?: string; work?: { toolName?: string } };

function freePort(): number {
	const probe = Bun.serve({ port: 0, fetch: () => new Response('') });
	const port = probe.port;
	probe.stop(true);
	if (port === undefined) throw new Error('the port probe bound no port');
	return port;
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
	const log = openSync(join(workDirectory, 'gateway.log'), 'a');
	gateway = Bun.spawn(
		[
			'bunx', 'wrangler', 'dev', '--local', '--port', String(gatewayPort), '--inspector-port', '0',
			'--persist-to', join(workDirectory, 'gateway-state'), '--config', configurationPath
		],
		{ cwd: join(repositoryRoot, 'web'), stdout: log, stderr: log }
	);
	for (let attempt = 0; attempt < 480; attempt += 1) {
		if ((await fetch(`http://127.0.0.1:${gatewayPort}/missing`).catch(() => null))?.status === 404) return;
		if (gateway.exitCode !== null) throw new Error(`the gateway exited with ${gateway.exitCode}; see ${workDirectory}/gateway.log`);
		await Bun.sleep(250);
	}
	throw new Error(`the gateway never answered; see ${workDirectory}/gateway.log`);
}

async function aClaimedBox(intent: 'replace' | 'add', address: string): Promise<string> {
	const { publicKey } = await generateKeyPair('EdDSA', { crv: 'Ed25519' });
	const { x } = await exportJWK(publicKey);
	if (!x) throw new Error('an Ed25519 key exports its public half as x');
	const encryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
	const announced = await announceBox(client, { publicKey: x, encryptionKey, publicAddress: address, wantsPairingCode: true });
	if (announced.isClaimed || !announced.pairingCode) throw new Error('an empty box that asks for a code is given one');
	const { ticket } = await verifyBoxCode(client, companyID, x, announced.pairingCode);
	await claimBox(client, companyID, x, ticket, intent);
	return x;
}

async function sessionOf(publicKey: string): Promise<{ role: string; accessToken: string }> {
	const session = await boxSessionFor(credentials, publicKey, environment, appURL);
	if (!session) throw new Error('a claimed box gets a session');
	return { role: session.role, accessToken: session.session.accessToken };
}

async function worker(publicKey: string, free: number, answer: (frame: Frame) => unknown): Promise<WebSocket> {
	const { accessToken } = await sessionOf(publicKey);
	const socket = new WebSocket(`ws://127.0.0.1:${gatewayPort}/company/${companyID}/worker`, {
		headers: { Authorization: `Bearer ${accessToken}` }
	} as unknown as string[]);
	await new Promise<void>((opened, refused) => {
		socket.addEventListener('open', () => opened(), { once: true });
		socket.addEventListener('error', () => refused(new Error('the worker socket was refused')), { once: true });
	});
	socket.addEventListener('message', (event) => {
		const frame: Frame = JSON.parse(String(event.data));
		if (frame.kind !== 'work' || !frame.requestID) return;
		socket.send(JSON.stringify({ kind: 'result', requestID: frame.requestID, status: 200, body: answer(frame) }));
	});
	socket.send(JSON.stringify({ kind: 'ready', free }));
	await Bun.sleep(200);
	return socket;
}

async function postWork(publicKey: string, affinityKey: string): Promise<{ httpStatus: number; status?: number; body?: unknown }> {
	const { accessToken } = await sessionOf(publicKey);
	const response = await fetch(`http://127.0.0.1:${gatewayPort}/company/${companyID}/work`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ requestID: crypto.randomUUID(), affinityKey, work: { toolName: 'browser_open' } })
	});
	if (!response.ok) return { httpStatus: response.status };
	const answered: { status: number; body: unknown } = await response.json();
	return { httpStatus: response.status, status: answered.status, body: answered.body };
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Boxes Share Work', slug: `boxes-share-work-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`boxes-share-work-${stamp}-admin@example.test`
	);
	companyID = provisioned.companyID;
	headOffice = await aClaimedBox('replace', `198.51.100.${stamp % 200}`);
	annex = await aClaimedBox('add', `203.0.113.${stamp % 200}`);
	await startTheGateway();
}, startupMilliseconds);

afterAll(async () => {
	gateway?.kill('SIGTERM');
	if (gateway) await Promise.race([gateway.exited, Bun.sleep(3000)]);
	gateway?.kill('SIGKILL');
	if (companyID) await client.from('company').delete().eq('id', companyID);
}, startupMilliseconds);

test('work the service box puts in the queue is answered by the other box, which named only its room', async () => {
	expect((await sessionOf(headOffice)).role).toBe('service');
	expect((await sessionOf(annex)).role).toBe('standby');
	const annexWorker = await worker(annex, 1, () => ({ content: 'opened on the annex' }));

	const answered = await postWork(headOffice, 'browser:sample@example.com');

	expect(answered).toMatchObject({ httpStatus: 200, status: 200, body: { content: 'opened on the annex' } });
	annexWorker.close();
}, startupMilliseconds);

test('a box that only stands by cannot put work in the queue', async () => {
	expect((await postWork(annex, 'browser:sample@example.com')).httpStatus).toBe(409);
}, startupMilliseconds);

test('work no box has room for is answered that no box had room', async () => {
	const fullWorker = await worker(annex, 0, () => ({ content: 'should not run' }));

	const answered = await postWork(headOffice, 'browser:sample@example.com');

	expect(answered).toMatchObject({ httpStatus: 200, status: 503 });
	fullWorker.close();
}, startupMilliseconds);

test('when the administrator makes the other box serve, the queue follows the new service box', async () => {
	const keptManifest = `${companyID}/snapshots/internkim-snapshot-20261020T000000Z.json`;
	await client.storage.from('box_backup').upload(keptManifest, new Blob(['{}'], { type: 'application/json' }));
	expect(await makeBoxServe(client, companyID, annex)).toBe('serving');
	await client.storage.from('box_backup').remove([keptManifest]);
	expect((await sessionOf(annex)).role).toBe('service');
	expect((await sessionOf(headOffice)).role).toBe('standby');
	const headOfficeWorker = await worker(headOffice, 1, () => ({ content: 'opened at the head office' }));

	expect((await postWork(headOffice, 'browser:sample@example.com')).httpStatus).toBe(409);
	expect(await postWork(annex, 'browser:sample@example.com')).toMatchObject({
		httpStatus: 200,
		status: 200,
		body: { content: 'opened at the head office' }
	});
	headOfficeWorker.close();
}, startupMilliseconds);

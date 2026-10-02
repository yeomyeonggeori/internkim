import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { mkdtemp, open, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { ServerWebSocket, Subprocess } from 'bun';
import {
	addMember,
	asMember,
	controlPlane,
	issueAgentKey,
	provisionCompany,
	sessionForHost,
	sessionForMember,
	sessionForPlatformIdentity
} from '../../src/lib/server/control-plane';
import type { CompanyEvent } from '../../src/lib/company-event';
import { transferThroughTheHost, type HostConnection } from '../../src/lib/transfer/host-transfer';
import { uploadToStore } from '../../src/lib/transfer/store-upload';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../integration/supabase-environment';

const mebibyte = 1024 * 1024;
const fileBytes = 320 * mebibyte;
const boundedGrowthBytes = fileBytes * 0.75;
const rangeBytes = 6 * mebibyte;
const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };
const client = controlPlane(credentials);
const stamp = Date.now();
const relayEntry = join(import.meta.dir, '../../../host/relay/relay.ts');
const transferTimeout = 600_000;

type Member = { id: string; email: string; accessToken: string; workspaceHome: string };

let companyID = '';
let agentKey = '';
let owner: Member;
let colleague: Member;
let directory = '';
let sourcePath = '';
let sourceDigest = '';
let sourceModifiedAt = '';
let relay: Subprocess | null = null;
const peakResidentBytes = { baseline: 0, peak: 0 };
let residentSampler: ReturnType<typeof setInterval> | undefined;

type Servers = { app: ReturnType<typeof Bun.serve>; chatd: ReturnType<typeof Bun.serve>; admind: ReturnType<typeof Bun.serve>; gateway: ReturnType<typeof Bun.serve> };
let servers: Servers;

const chatdRangesAsked: string[] = [];
const chatdUploads: { digest: string; sizeBytes: number; actor: unknown }[] = [];
const admindAsked: { method: string; path: string; requester: string; range: string }[] = [];
const workspaceWrites: { path: string; requester: string; digest: string; sizeBytes: number }[] = [];

function chunkAt(index: number): Uint8Array<ArrayBuffer> {
	const chunk = new Uint8Array(mebibyte);
	for (let offset = 0; offset < chunk.length; offset += 1) chunk[offset] = (offset * 131 + index * 17) & 0xff;
	return chunk;
}

async function writeSourceFile(path: string): Promise<string> {
	const hasher = new Bun.CryptoHasher('sha256');
	const file = await open(path, 'w');
	for (let index = 0; index * mebibyte < fileBytes; index += 1) {
		const chunk = chunkAt(index);
		hasher.update(chunk);
		await file.write(chunk);
	}
	await file.close();
	return hasher.digest('hex');
}

function rangeOf(request: Request, sizeBytes: number, source: string): Response {
	const asked = /^bytes=(\d+)-(\d+)$/.exec(request.headers.get('range') ?? '');
	if (!asked) return new Response('this fake serves ranges only, as the real one is asked for', { status: 400 });
	const start = Number(asked[1]);
	const end = Math.min(Number(asked[2]), sizeBytes - 1);
	if (start >= sizeBytes) return new Response(null, { status: 416, headers: { 'content-range': `bytes */${sizeBytes}` } });
	return new Response(Bun.file(source).slice(start, end + 1), {
		status: 206,
		headers: { 'content-range': `bytes ${start}-${end}/${sizeBytes}`, 'content-type': 'video/mp4' }
	});
}

async function digestOfRanges(fetchRange: (range: string) => Promise<Response>): Promise<{ digest: string; sizeBytes: number }> {
	const hasher = new Bun.CryptoHasher('sha256');
	let sizeBytes = 0;
	for (;;) {
		const response = await fetchRange(`bytes=${sizeBytes}-${sizeBytes + rangeBytes - 1}`);
		if (response.status === 416) break;
		if (response.status !== 206) throw new Error(`the store answered ${response.status} for a range`);
		const bytes = new Uint8Array(await response.arrayBuffer());
		hasher.update(bytes);
		sizeBytes += bytes.byteLength;
		const total = Number(/\/(\d+)$/.exec(response.headers.get('content-range') ?? '')?.[1] ?? 0);
		if (bytes.byteLength === 0 || sizeBytes >= total) break;
	}
	return { digest: hasher.digest('hex'), sizeBytes };
}

function serveApp(): ReturnType<typeof Bun.serve> {
	return Bun.serve({
		port: 0,
		hostname: '127.0.0.1',
		async fetch(request) {
			const url = new URL(request.url);
			const presented = (request.headers.get('authorization') ?? '').replace(/^Bearer /, '');
			if (url.pathname === '/api/agent/host-session') return Response.json(await sessionForHost(credentials, presented));
			if (url.pathname === '/api/agent/session') {
				const asked = (await request.json()) as { kind: string; externalID: string };
				return Response.json(await sessionForPlatformIdentity(credentials, presented, asked.kind, asked.externalID));
			}
			if (url.pathname === '/api/agent/messenger-credential') {
				return Response.json({ credential: { kind: 'buzz-secret', secret: `secret-of-${url.searchParams.get('memberID')}` } });
			}
			if (url.pathname === '/api/agent/messenger-accounts-reconcile') return Response.json({ reconciled: [] });
			return new Response('not part of this test', { status: 404 });
		}
	});
}

async function digestOfSignedSource(sourceURL: string): Promise<{ digest: string; sizeBytes: number }> {
	return digestOfRanges((range) => {
		chatdRangesAsked.push(range);
		return fetch(sourceURL, { headers: { Range: range } });
	});
}

function serveChatd(): ReturnType<typeof Bun.serve> {
	return Bun.serve({
		port: 0,
		hostname: '127.0.0.1',
		async fetch(request) {
			const capability = new URL(request.url).pathname.split('/').pop() ?? '';
			const body = (await request.json().catch(() => ({}))) as Record<string, unknown>;
			if (capability === 'person.credential.requirement') return Response.json({ credentialKind: 'buzz-secret' });
			if (capability === 'person.media.read') {
				chatdRangesAsked.push(String(body.range));
				const forged = new Request('http://chatd.test', { headers: { range: String(body.range) } });
				return rangeOf(forged, fileBytes, sourcePath);
			}
			if (capability === 'person.media.upload') {
				const read = await digestOfSignedSource(String(body.sourceURL));
				chatdUploads.push({ ...read, actor: body.actor });
				return Response.json({
					media: { address: `http://media.test/${read.digest}.mp4`, digest: read.digest, sizeBytes: read.sizeBytes, contentType: body.contentType }
				});
			}
			return Response.json({});
		}
	});
}

function workspaceOwnerOf(path: string): Member | null {
	if (path.startsWith(owner.workspaceHome)) return owner;
	if (path.startsWith(colleague.workspaceHome)) return colleague;
	return null;
}

function serveAdmind(socketPath: string): ReturnType<typeof Bun.serve> {
	return Bun.serve({
		unix: socketPath,
		async fetch(request) {
			const url = new URL(request.url);
			const requester = request.headers.get('x-internkim-requester-email') ?? '';
			const path = url.searchParams.get('path') ?? '';
			admindAsked.push({ method: request.method, path: url.pathname, requester, range: request.headers.get('range') ?? '' });
			if (workspaceOwnerOf(path)?.email !== requester) {
				return Response.json({ error: 'workspace path is not accessible' }, { status: 403 });
			}
			if (url.pathname === '/files/api/list') {
				return Response.json({ entries: [{ name: 'film.mov', isDirectory: false, size: fileBytes, modifiedAt: sourceModifiedAt }] });
			}
			if (url.pathname === '/files/api/download') return rangeOf(request, fileBytes, sourcePath);
			if (url.pathname === '/files/api/file' && request.method === 'PUT' && request.body) {
				const hasher = new Bun.CryptoHasher('sha256');
				let sizeBytes = 0;
				const reader = request.body.getReader();
				for (let read = await reader.read(); !read.done; read = await reader.read()) {
					hasher.update(read.value);
					sizeBytes += read.value.byteLength;
				}
				workspaceWrites.push({ path, requester, digest: hasher.digest('hex'), sizeBytes });
				return Response.json({ path, sizeBytes });
			}
			return new Response('not part of this test', { status: 404 });
		}
	});
}

type Frame = Record<string, unknown>;
let hostSocket: ServerWebSocket<unknown> | null = null;
let hostConnected: () => void = () => undefined;
const hostIsConnected = new Promise<void>((resolve) => (hostConnected = resolve));
const answers = new Map<string, (frame: Frame) => void>();
const listeners = new Set<(event: CompanyEvent, audience: string[]) => void>();

function serveGateway(): ReturnType<typeof Bun.serve> {
	return Bun.serve({
		port: 0,
		hostname: '127.0.0.1',
		fetch(request, server) {
			if (server.upgrade(request, { data: undefined })) return undefined;
			return new Response('a socket is all this fake gateway serves', { status: 400 });
		},
		websocket: {
			open(socket) {
				hostSocket = socket;
				hostConnected();
			},
			message(_socket, data) {
				const frame = JSON.parse(String(data)) as Frame;
				if (frame.kind === 'result') answers.get(String(frame.requestID))?.(frame);
				if (frame.kind === 'deliver') {
					const audience = Array.isArray(frame.audienceMemberIDs) ? frame.audienceMemberIDs.map(String) : [];
					for (const listener of listeners) listener(frame.event as CompanyEvent, audience);
				}
			}
		}
	});
}

function browserOf(member: Member): HostConnection {
	return {
		call: (call) =>
			new Promise((resolve) => {
				const requestID = crypto.randomUUID();
				answers.set(requestID, (frame) => resolve({ status: Number(frame.status), body: frame.body }));
				hostSocket?.send(JSON.stringify({ kind: 'call', requestID, memberID: member.id, capability: call.capability, body: call.body ?? {} }));
			}),
		listen: (listener) => {
			const forMember = (event: CompanyEvent, audience: string[]) => {
				if (audience.includes(member.id)) listener(event);
			};
			listeners.add(forMember);
			return () => listeners.delete(forMember);
		}
	};
}

function storeSessionOf(member: Member) {
	return { projectURL, publishableKey, accessToken: async () => member.accessToken, companyID, memberID: member.id };
}

function pathOfAddress(address: string): string {
	return address.slice(`${projectURL.replace(/\/+$/, '')}/storage/v1/object/asset/`.length);
}

async function readAsMember(member: Member, address: string): Promise<{ digest: string; sizeBytes: number }> {
	const signed = await asMember(credentials, member.accessToken).storage.from('asset').createSignedUrl(pathOfAddress(address), 600);
	if (signed.error || !signed.data) throw new Error(`the store would not sign ${address} for ${member.email}: ${signed.error?.message}`);
	return digestOfRanges((range) => fetch(signed.data.signedUrl, { headers: { Range: range } }));
}

async function memberOf(email: string, workspaceHome: string): Promise<Member> {
	const id = await addMember(client, companyID, email);
	await client.from('member').update({ status: 'active' }).eq('id', id);
	const session = await sessionForMember(credentials, id);
	return { id, email, accessToken: session.accessToken, workspaceHome };
}

function residentBytesOf(processID: number): number {
	const answered = Bun.spawnSync(['ps', '-o', 'rss=', '-p', String(processID)]);
	return Number(new TextDecoder().decode(answered.stdout).trim()) * 1024;
}

async function startRelay(): Promise<void> {
	const stateDirectory = join(directory, 'relay-state');
	relay = Bun.spawn([process.execPath, 'run', relayEntry], {
		cwd: join(relayEntry, '..'),
		env: {
			...process.env,
			SUPABASE_URL: projectURL,
			SUPABASE_PUBLISHABLE_KEY: publishableKey,
			AGENT_API_KEY: agentKey,
			INTERNKIM_APP_URL: `http://127.0.0.1:${servers.app.port}`,
			MESSENGER_PLATFORM: 'buzz',
			CHATD_BASE_URL: `http://127.0.0.1:${servers.chatd.port}`,
			ADMIND_SOCKET_PATH: join(directory, 'admind.sock'),
			GATEWAY_URL: `http://127.0.0.1:${servers.gateway.port}`,
			ARRIVALS_PORT: String(40_000 + Math.floor(Math.random() * 20_000)),
			RELAY_STATE_DIR: stateDirectory,
			BLUECLAW_ACP_SOCKET_PATH: join(directory, 'no-agent.sock'),
			WORKSPACE_ROOT_PATH: directory
		},
		stdout: 'inherit',
		stderr: 'inherit'
	});
	await hostIsConnected;
}

beforeAll(async () => {
	directory = await mkdtemp(join(tmpdir(), 'relay-file-transfer-'));
	sourcePath = join(directory, 'film.mov');
	sourceDigest = await writeSourceFile(sourcePath);
	sourceModifiedAt = (await stat(sourcePath)).mtime.toISOString();

	const company = await provisionCompany(
		client,
		{ name: 'Transfer', slug: `transfer-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`transfer-${stamp}-admin@example.test`
	);
	companyID = company.companyID;
	agentKey = (await issueAgentKey(client, companyID, 'transfer-host')).apiKey;
	owner = await memberOf(`transfer-owner-${stamp}@example.test`, '/workspace/private/people/person-owner');
	colleague = await memberOf(`transfer-colleague-${stamp}@example.test`, '/workspace/private/people/person-colleague');

	servers = { app: serveApp(), chatd: serveChatd(), admind: serveAdmind(join(directory, 'admind.sock')), gateway: serveGateway() };
	await startRelay();
	await warmUp();
	peakResidentBytes.baseline = residentBytesOf(relay?.pid ?? 0);
	peakResidentBytes.peak = peakResidentBytes.baseline;
	residentSampler = setInterval(() => {
		peakResidentBytes.peak = Math.max(peakResidentBytes.peak, residentBytesOf(relay?.pid ?? 0));
	}, 100);
}, transferTimeout);

async function warmUp(): Promise<void> {
	const small = new Blob([chunkAt(0)]);
	const object = await uploadToStore(storeSessionOf(owner), small, 'application/octet-stream');
	await transferThroughTheHost(browserOf(owner), {
		capability: 'person.files.upload',
		body: { object, directoryPath: owner.workspaceHome, filename: 'warm.bin' }
	});
	workspaceWrites.length = 0;
}

afterAll(async () => {
	clearInterval(residentSampler);
	relay?.kill();
	await relay?.exited;
	for (const server of Object.values(servers ?? {})) server.stop(true);
	if (companyID) {
		const { data: objects } = await client.storage.from('asset').list(`${companyID}/shared/transfer`);
		const kept = (objects ?? []).map((one) => `${companyID}/shared/transfer/${one.name}`);
		for (const member of [owner, colleague].filter(Boolean)) {
			for (const kind of ['transfer', 'upload']) {
				const { data } = await client.storage.from('asset').list(`${companyID}/person/${member.id}/${kind}`);
				kept.push(...(data ?? []).map((one) => `${companyID}/person/${member.id}/${kind}/${one.name}`));
			}
		}
		if (kept.length > 0) await client.storage.from('asset').remove(kept);
		const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
		await client.from('company').delete().eq('id', companyID);
		for (const member of members ?? []) if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
	await rm(directory, { recursive: true, force: true });
	console.log(
		`relay resident memory: ${(peakResidentBytes.baseline / mebibyte).toFixed(1)} MiB at rest, ${(peakResidentBytes.peak / mebibyte).toFixed(1)} MiB at its peak, moving ${fileBytes / mebibyte} MiB files`
	);
}, transferTimeout);

function readableCopy(result: unknown): { address: string; sizeBytes: number } {
	const copy = result as { address?: unknown; sizeBytes?: unknown };
	if (typeof copy.address !== 'string') throw new Error(`no address in ${JSON.stringify(result)}`);
	return { address: copy.address, sizeBytes: Number(copy.sizeBytes) };
}

describe('a file larger than the old frame, base64 and bucket limits, through the real relay and store', () => {
	test('a messenger attachment is copied for its reader and opens unchanged', async () => {
		const progress: number[] = [];
		const copy = readableCopy(
			await transferThroughTheHost(
				browserOf(owner),
				{
					capability: 'person.media.prepare',
					body: { mediaURL: `http://media.test/${sourceDigest}.mp4`, digest: sourceDigest, contentType: 'video/mp4' }
				},
				(copied) => progress.push(copied)
			)
		);

		expect(copy.sizeBytes).toBe(fileBytes);
		expect(progress.length).toBeGreaterThan(0);
		expect(await readAsMember(colleague, copy.address)).toEqual({ digest: sourceDigest, sizeBytes: fileBytes });
		expect(chatdRangesAsked.every((range) => /^bytes=\d+-\d+$/.test(range))).toBe(true);
	}, transferTimeout);

	test('the same attachment again is answered at once, without the messenger being read', async () => {
		chatdRangesAsked.length = 0;
		const copy = readableCopy(
			await transferThroughTheHost(browserOf(colleague), {
				capability: 'person.media.prepare',
				body: { mediaURL: `http://media.test/${sourceDigest}.mp4`, digest: sourceDigest, contentType: 'video/mp4' }
			})
		);

		expect(copy.sizeBytes).toBe(fileBytes);
		expect(chatdRangesAsked).toEqual([]);
	}, transferTimeout);

	test('an attachment whose bytes are not the digest it was asked by is refused, and nothing is kept under that digest', async () => {
		const forged = 'f'.repeat(64);
		const failure = await transferThroughTheHost(browserOf(colleague), {
			capability: 'person.media.prepare',
			body: { mediaURL: `http://media.test/${forged}.mp4`, digest: forged, contentType: 'video/mp4' }
		}).catch((thrown: unknown) => thrown);

		expect(failure).toMatchObject({ status: 422 });
		const { data } = await client.storage.from('asset').list(`${companyID}/shared/transfer`, { search: forged });
		expect(data ?? []).toEqual([]);
	}, transferTimeout);

	test('a workspace file is read as its owner, copied where only the owner reads, and opens unchanged', async () => {
		admindAsked.length = 0;
		const path = `${owner.workspaceHome}/film.mov`;
		const copy = readableCopy(
			await transferThroughTheHost(browserOf(owner), { capability: 'person.files.download', body: { path } })
		);

		expect(await readAsMember(owner, copy.address)).toEqual({ digest: sourceDigest, sizeBytes: fileBytes });
		expect(admindAsked.every((asked) => asked.requester === owner.email)).toBe(true);
		expect(admindAsked.filter((asked) => asked.path === '/files/api/download').every((asked) => /^bytes=\d+-\d+$/.test(asked.range))).toBe(true);
		await expect(readAsMember(colleague, copy.address)).rejects.toThrow('would not sign');
	}, transferTimeout);

	test('another member cannot have somebody else\'s private file copied for them', async () => {
		admindAsked.length = 0;
		const failure = await transferThroughTheHost(browserOf(colleague), {
			capability: 'person.files.download',
			body: { path: `${owner.workspaceHome}/film.mov` }
		}).catch((thrown: unknown) => thrown);

		expect(failure).toMatchObject({ status: 403 });
		expect(admindAsked.map((asked) => [asked.path, asked.requester])).toEqual([['/files/api/list', colleague.email]]);
	}, transferTimeout);

	test('a file sent with a message goes from the browser to the store, then on to the messenger, unchanged', async () => {
		chatdUploads.length = 0;
		const object = await uploadToStore(storeSessionOf(owner), Bun.file(sourcePath), 'video/mp4');
		const result = (await transferThroughTheHost(browserOf(owner), {
			capability: 'person.media.upload',
			body: { object, filename: 'film.mov', contentType: 'video/mp4' }
		})) as { attachment: { address: string; digest: string; sizeBytes: number; filename: string } };

		expect(result.attachment).toEqual({
			filename: 'film.mov',
			contentType: 'video/mp4',
			address: `http://media.test/${sourceDigest}.mp4`,
			digest: sourceDigest,
			sizeBytes: fileBytes
		} as never);
		expect(chatdUploads).toEqual([{ digest: sourceDigest, sizeBytes: fileBytes, actor: { kind: 'buzz-secret', secret: `secret-of-${owner.id}` } }]);
		const { data } = await client.storage.from('asset').list(object.slice(0, object.lastIndexOf('/')));
		expect(data ?? []).toEqual([]);
	}, transferTimeout);

	test('a file added on the Files screen lands in the owner\'s workspace, written as the owner, unchanged', async () => {
		const object = await uploadToStore(storeSessionOf(owner), Bun.file(sourcePath), 'video/mp4');
		const result = (await transferThroughTheHost(browserOf(owner), {
			capability: 'person.files.upload',
			body: { object, directoryPath: owner.workspaceHome, filename: '../film.mov' }
		})) as { file: { path: string; sizeBytes: number } };

		expect(result.file).toEqual({ path: `${owner.workspaceHome}/film.mov`, sizeBytes: fileBytes });
		expect(workspaceWrites).toEqual([
			{ path: `${owner.workspaceHome}/film.mov`, requester: owner.email, digest: sourceDigest, sizeBytes: fileBytes }
		]);
	}, transferTimeout);

	test('an upload another member put in the store cannot be taken by somebody else', async () => {
		const object = await uploadToStore(storeSessionOf(owner), new Blob([chunkAt(1)]), 'application/octet-stream');
		workspaceWrites.length = 0;

		const intoTheirWorkspace = await transferThroughTheHost(browserOf(colleague), {
			capability: 'person.files.upload',
			body: { object, directoryPath: colleague.workspaceHome, filename: 'taken.bin' }
		}).catch((thrown: unknown) => thrown);
		const intoTheMessenger = await transferThroughTheHost(browserOf(colleague), {
			capability: 'person.media.upload',
			body: { object, filename: 'taken.bin', contentType: 'application/octet-stream' }
		}).catch((thrown: unknown) => thrown);

		expect(intoTheirWorkspace).toMatchObject({ status: 404 });
		expect(intoTheMessenger).toMatchObject({ status: 404 });
		expect(workspaceWrites).toEqual([]);
	}, transferTimeout);

	test('the relay never held a file: its memory stayed well under one file however many it moved', () => {
		const growthBytes = peakResidentBytes.peak - peakResidentBytes.baseline;
		console.log(`the relay grew by ${(growthBytes / mebibyte).toFixed(1)} MiB at its peak moving ${fileBytes / mebibyte} MiB four ways`);
		expect(growthBytes).toBeLessThan(boundedGrowthBytes);
	});
});

import { afterEach, describe, expect, test } from 'bun:test';
import {
	prepareMedia,
	prepareWorkspaceFile,
	takeUploadIntoMessenger,
	takeUploadIntoWorkspace,
	type FileTransferDependencies
} from './file-transfer';
import { Transfers } from './transfers';

const realFetch = globalThis.fetch;
const projectURL = 'https://company.supabase.test';
const companyID = '43000000-0000-0000-0000-0000000000a0';
const actor = { kind: 'buzz-secret', secret: 'a-held-secret' };
const digest = 'c'.repeat(64);

afterEach(() => {
	globalThis.fetch = realFetch;
});

function storeHolding(sizes: Record<string, number>, signable: string[] = []): string[] {
	const asked: string[] = [];
	globalThis.fetch = (async (input: RequestInfo | URL) => {
		const url = String(input);
		asked.push(url);
		const info = url.split('/storage/v1/object/info/authenticated/asset/')[1];
		if (info !== undefined) {
			return info in sizes ? Response.json({ size: sizes[info] }) : Response.json({ statusCode: '404' }, { status: 400 });
		}
		const signed = url.split('/storage/v1/object/sign/asset/')[1];
		if (signed !== undefined) {
			return signable.includes(signed)
				? Response.json({ signedURL: `/object/sign/asset/${signed}?token=t` })
				: Response.json({ statusCode: '404', error: 'not_found' }, { status: 400 });
		}
		return new Response('unexpected', { status: 599 });
	}) as typeof fetch;
	return asked;
}

function dependencies(overrides: Partial<FileTransferDependencies> = {}) {
	const started: string[] = [];
	const transfers = new Transfers(() => undefined);
	const original = transfers.start.bind(transfers);
	transfers.start = (key, watcher, work) => {
		started.push(key);
		original(key, watcher, async () => null);
	};
	const access = { projectURL, apiKey: 'publishable', accessToken: async () => 'token' };
	const neverAsked = async (): Promise<never> => {
		throw new Error('nothing should have been read');
	};
	const values: FileTransferDependencies = {
		companyID,
		hostAccess: access,
		memberAccess: () => access,
		transfers,
		readMediaRange: neverAsked,
		uploadMedia: neverAsked,
		listWorkspaceDirectory: async () => ({ status: 200, body: { entries: [] } }),
		readWorkspaceRange: neverAsked,
		writeWorkspaceFile: neverAsked,
		report: () => undefined,
		...overrides
	};
	return { values, started };
}

describe('preparing a messenger file for reading', () => {
	test('one already copied is answered at once, without the messenger being read', async () => {
		storeHolding({ [`${companyID}/shared/transfer/${digest}.png`]: 91_000_000 });
		const { values, started } = dependencies();

		const answer = await prepareMedia(values, 'member-1', actor, {
			transferID: 'transfer-0001',
			mediaURL: `https://relay.test/media/${digest}.png`,
			digest,
			contentType: 'image/png'
		});

		expect(answer).toEqual({
			status: 200,
			body: {
				transfer: {
					state: 'ready',
					address: `${projectURL}/storage/v1/object/asset/${companyID}/shared/transfer/${digest}.png`,
					sizeBytes: 91_000_000
				}
			}
		});
		expect(started).toEqual([]);
	});

	test('one not yet copied is answered as pending, and copied behind the answer', async () => {
		storeHolding({});
		const { values, started } = dependencies();

		const answer = await prepareMedia(values, 'member-1', actor, {
			transferID: 'transfer-0001',
			mediaURL: `https://relay.test/media/${digest}.mp4`,
			digest,
			contentType: 'video/mp4'
		});

		expect(answer).toEqual({ status: 200, body: { transfer: { state: 'pending' } } });
		expect(started).toEqual([`${companyID}/shared/transfer/${digest}`]);
	});

	test('a call without an id to answer it by is refused', async () => {
		storeHolding({});
		const { values, started } = dependencies();

		const answer = await prepareMedia(values, 'member-1', actor, { mediaURL: 'https://relay.test/media/x.png' });

		expect(answer.status).toBe(400);
		expect(started).toEqual([]);
	});
});

describe('preparing a workspace file for reading', () => {
	test('a file the person may not list is refused with what the workspace said, and nothing is copied', async () => {
		storeHolding({});
		const { values, started } = dependencies({
			listWorkspaceDirectory: async () => ({ status: 403, body: { error: 'workspace path is not accessible' } })
		});

		const answer = await prepareWorkspaceFile(values, 'member-2', 'other@example.test', {
			transferID: 'transfer-0002',
			path: '/workspace/private/people/person-1/salary.xlsx'
		});

		expect(answer).toEqual({ status: 403, body: { error: 'workspace path is not accessible' } });
		expect(started).toEqual([]);
	});

	test('a file that is not there is said to be not there', async () => {
		storeHolding({});
		const { values } = dependencies();

		const answer = await prepareWorkspaceFile(values, 'member-1', 'sample@example.test', {
			transferID: 'transfer-0002',
			path: '/workspace/private/people/person-1/gone.txt'
		});

		expect(answer.status).toBe(404);
	});

	test('is copied under the person who asked, which nobody else may read', async () => {
		storeHolding({});
		const { values, started } = dependencies({
			listWorkspaceDirectory: async () => ({
				status: 200,
				body: { entries: [{ name: 'deck.pptx', isDirectory: false, size: 70_000_000, modifiedAt: '2026-10-01T00:00:00Z' }] }
			})
		});

		await prepareWorkspaceFile(values, 'member-1', 'sample@example.test', {
			transferID: 'transfer-0002',
			path: '/workspace/private/people/person-1/deck.pptx'
		});

		expect(started).toHaveLength(1);
		expect(started[0]?.startsWith(`${companyID}/person/member-1/transfer/`)).toBe(true);
	});
});

describe('taking an upload the browser put in the store', () => {
	const object = `${companyID}/person/member-1/upload/1f0e`;

	test('into the workspace, refuses one the member may not read before anything is written', async () => {
		storeHolding({});
		const { values, started } = dependencies();

		const answer = await takeUploadIntoWorkspace(values, 'member-2', 'other@example.test', {
			transferID: 'transfer-0003',
			object,
			directoryPath: '/workspace/private/people/person-2',
			filename: 'copied.pdf'
		});

		expect(answer.status).toBe(404);
		expect(started).toEqual([]);
	});

	test('into the workspace, writes it under the leaf of the name it came with', async () => {
		storeHolding({ [object]: 12 });
		const { values, started } = dependencies();

		const answer = await takeUploadIntoWorkspace(values, 'member-1', 'sample@example.test', {
			transferID: 'transfer-0003',
			object,
			directoryPath: '/workspace/private/people/person-1/',
			filename: '../report.pdf'
		});

		expect(answer).toEqual({ status: 200, body: { transfer: { state: 'pending' } } });
		expect(started).toEqual(['upload\ntransfer-0003']);
	});

	test('into the messenger, refuses one the member may not sign for', async () => {
		storeHolding({});
		const { values, started } = dependencies();

		const answer = await takeUploadIntoMessenger(values, 'member-2', actor, 'other@example.test', {
			transferID: 'transfer-0004',
			object,
			filename: 'clip.mp4',
			contentType: 'video/mp4'
		});

		expect(answer.status).toBe(404);
		expect(started).toEqual([]);
	});
});

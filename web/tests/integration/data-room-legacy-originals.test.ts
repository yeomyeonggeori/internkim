import { createHash } from 'node:crypto';
import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { restoreLegacyOriginals } from '../../src/lib/server/data-room-legacy-originals';
import { projectURL, serviceRoleKey } from './supabase-environment';

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `legacy-originals-${Date.now()}`;
const original = new TextEncoder().encode('%PDF-1.7\nlegacy original\n');
const missing = new TextEncoder().encode('never uploaded');
let companyID = '';
let filedID = '';
let missingID = '';
let legacyPath = '';

function sha256Of(bytes: Uint8Array): string {
	return createHash('sha256').update(bytes).digest('hex');
}

async function unfiledDocument(title: string, sha256: string): Promise<string> {
	const id = crypto.randomUUID();
	const { error } = await client.from('company_document').insert({
		id, company_id: companyID, kind: 'internal', document_type: 'report', title, category_code: 'CR', sha256
	});
	if (error) throw new Error(error.message);
	return id;
}

async function upload(path: string, bytes: Uint8Array): Promise<void> {
	const { error } = await client.storage.from('asset').upload(path, bytes, { contentType: 'application/octet-stream' });
	if (error) throw new Error(`${path}: ${error.message}`);
}

async function storagePathOf(documentID: string): Promise<string | null> {
	const { data } = await client.from('company_document').select('storage_path').eq('id', documentID).single();
	return data?.storage_path ?? null;
}

async function exists(path: string): Promise<boolean> {
	const { data } = await client.storage.from('asset').download(path);
	return data !== null;
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Legacy Originals Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	const hash = sha256Of(original);
	legacyPath = `${companyID}/dataroom/1/${hash}`;
	await upload(legacyPath, original);
	await upload(`${legacyPath}/text.md`, new TextEncoder().encode('legacy original'));
	filedID = await unfiledDocument('Articles of incorporation', hash);
	missingID = await unfiledDocument('Never uploaded', sha256Of(missing));
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

describe('an original filed under the clearance layout', () => {
	test('is moved to its document folder and named again, once', async () => {
		const expected = `${companyID}/dataroom/C/CR/articles-of-incorporation.${filedID}.pdf`;

		const rehearsal = await restoreLegacyOriginals(client, { isDryRun: true });
		const planned = rehearsal.restored.find((restored) => restored.documentID === filedID);
		expect(planned?.to).toBe(expected);
		expect(planned?.derived.map((derived) => derived.to)).toEqual([`${companyID}/dataroom/C/CR/articles-of-incorporation.${filedID}.text.md`]);
		expect(await storagePathOf(filedID)).toBeNull();
		expect(await exists(legacyPath)).toBe(true);

		const restoration = await restoreLegacyOriginals(client, { isDryRun: false });
		expect(restoration.restored.map((restored) => restored.documentID)).toContain(filedID);
		expect(await storagePathOf(filedID)).toBe(expected);
		expect(await exists(expected)).toBe(true);
		expect(await exists(`${companyID}/dataroom/C/CR/articles-of-incorporation.${filedID}.text.md`)).toBe(true);
		expect(await exists(legacyPath)).toBe(false);

		const again = await restoreLegacyOriginals(client, { isDryRun: false });
		expect(again.restored.map((restored) => restored.documentID)).not.toContain(filedID);
	});

	test('a document whose bytes are nowhere stays unfiled and is reported', async () => {
		const restoration = await restoreLegacyOriginals(client, { isDryRun: true });
		expect(restoration.unmatched.map((document) => document.documentID)).toContain(missingID);
		expect(await storagePathOf(missingID)).toBeNull();
	});
});

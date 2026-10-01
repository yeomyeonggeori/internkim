import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { runToolOverTheRecord, previewToolOverTheRecord } = await import('../../src/lib/server/public-api/record');
const { buildCapabilityToolCatalog } = await import('../../src/lib/server/public-api/catalog/tools');
const { CapabilityAnsweredBy, protocolVersion } = await import('../../src/lib/server/public-api/catalog/protocol');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `company-ledger-${Date.now()}`;
const now = new Date('2026-09-04T00:00:00.000Z');

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Company Ledger Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;

	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);

	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);
	admin = await signedInMember(adminID, `${slug}-admin@example.test`);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

async function asSample(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(sample, client, sampleID, name, input, now, leavesTaskLabelsUndecided));
}

async function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(admin, client, adminID, name, input, now, leavesTaskLabelsUndecided));
}

function resultOf(answer: { body: unknown }): Record<string, unknown> {
	return (answer.body as { result: Record<string, unknown> }).result;
}

function errorOf(answer: { body: unknown }): string {
	return (answer.body as { error?: string }).error ?? '';
}

function candidatesOf(answer: { body: unknown }): { id: string; label: string }[] {
	return (answer.body as { candidates?: { id: string; label: string }[] }).candidates ?? [];
}

function descriptorOf(name: string) {
	const catalog = buildCapabilityToolCatalog(protocolVersion);
	const descriptor = catalog.tools.find((tool) => tool.name === name);
	if (!descriptor) throw new Error(`${name} is not in the catalog`);
	return descriptor;
}

const ledgerToolNames = [
	'company_metric_record',
	'company_metric_list',
	'company_record_add',
	'company_record_list',
	'company_record_update',
	'company_record_delete',
	'company_document_register',
	'company_document_list',
	'company_document_search',
	'company_document_update',
	'company_document_upload',
	'company_document_download'
];

describe('the ledger is the record answering', () => {
	test('every ledger tool says the record answers it and carries a typed result', () => {
		for (const name of ledgerToolNames) {
			const descriptor = descriptorOf(name);
			expect(descriptor.answeredBy).toBe(CapabilityAnsweredBy.Record);
			expect(descriptor.resultContract).toBeDefined();
		}
	});
});

describe('what the company states about itself', () => {
	test('is an administrator to record, and the record says so to anybody else', async () => {
		const refused = await asSample('company_metric_record', {
			metric: 'annualRevenue',
			year: 2025,
			value: 1_200_000_000,
			currency: 'KRW',
			valueUSD: 870_000
		});

		expect(refused.status).toBe(403);
		expect(errorOf(refused)).toContain('administrator');
	});

	test('records a number with the USD equivalent a reader can compare', async () => {
		const written = await asAdmin('company_metric_record', {
			metric: 'annualRevenue',
			year: 2025,
			value: 1_200_000_000,
			currency: 'krw',
			valueUSD: 870_000
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).currency).toBe('KRW');
		expect(resultOf(written).valueUSD).toBe(870_000);
		expect(resultOf(written).quarter).toBe(0);
	});

	test('refuses money whose USD equivalent nobody gave', async () => {
		const refused = await asAdmin('company_metric_record', {
			metric: 'operatingProfit',
			year: 2025,
			value: 1_000,
			currency: 'KRW'
		});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('USD equivalent');
	});

	test('refuses a period that is quarterly and monthly at once', async () => {
		const refused = await asAdmin('company_metric_record', {
			metric: 'mau',
			year: 2025,
			quarter: 2,
			month: 3,
			value: 9_000
		});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('never both');
	});

	test('writes the same period again rather than a second row', async () => {
		await asAdmin('company_metric_record', {
			metric: 'annualRevenue',
			year: 2025,
			value: 1_300_000_000,
			currency: 'USD',
			valueUSD: 1_300_000_000
		});
		const listed = await asAdmin('company_metric_list', { metric: 'annualRevenue' });

		expect(listed.status).toBe(200);
		expect(resultOf(listed).count).toBe(1);
		expect((resultOf(listed).metrics as { value: number }[])[0].value).toBe(1_300_000_000);
	});

	test('is read by anybody who works here, and filtered by year', async () => {
		await asAdmin('company_metric_record', { metric: 'mau', year: 2024, month: 12, value: 9_000, unit: 'people' });

		const everything = await asSample('company_metric_list');
		const recent = await asSample('company_metric_list', { fromYear: 2025 });

		expect(resultOf(everything).count).toBe(2);
		expect(resultOf(recent).count).toBe(1);
		expect((resultOf(recent).metrics as { metric: string }[])[0].metric).toBe('annualRevenue');
	});
});

describe('what happened to the company', () => {
	let seedRecordID = '';

	test('is an administrator to add, and the record says so to anybody else', async () => {
		const refused = await asSample('company_record_add', { category: 'award', title: 'Prize nobody gave us' });

		expect(refused.status).toBe(403);
		expect(errorOf(refused)).toContain('administrator');
	});

	test('keeps the structured details the call names', async () => {
		const added = await asAdmin('company_record_add', {
			category: 'funding',
			date: '2025-12-01',
			title: 'Seed round closed',
			detail: 'Capital secured for product validation.',
			attributes: '{"round": "Seed", "investors": "ABC Ventures"}'
		});

		expect(added.status).toBe(200);
		expect(resultOf(added).attributes).toEqual([
			{ label: 'round', value: 'Seed' },
			{ label: 'investors', value: 'ABC Ventures' }
		]);
		seedRecordID = resultOf(added).recordID as string;
	});

	test('refuses details that are not a JSON object string', async () => {
		const refused = await asAdmin('company_record_add', {
			category: 'award',
			title: 'Award with broken details',
			attributes: 'round: Seed'
		});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('JSON object string');
	});

	test('is listed newest first and filtered by category and keyword', async () => {
		await asAdmin('company_record_add', { category: 'award', date: '2026-01-05', title: 'Design award 2025' });

		const everything = await asSample('company_record_list');
		const funding = await asSample('company_record_list', { category: 'funding' });
		const searched = await asSample('company_record_list', { query: 'ABC Ventures' });

		expect(resultOf(everything).count).toBe(2);
		expect((resultOf(everything).records as { title: string }[])[0].title).toBe('Design award 2025');
		expect(resultOf(funding).count).toBe(1);
		expect((resultOf(searched).records as { recordID: string }[])[0].recordID).toBe(seedRecordID);
	});

	test('is named by its title as readily as by its id', async () => {
		const written = await asAdmin('company_record_update', {
			recordHint: 'Seed round closed',
			title: 'Pre-seed round closed'
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).recordID).toBe(seedRecordID);
		expect(resultOf(written).title).toBe('Pre-seed round closed');
		expect(resultOf(written).category).toBe('funding');
	});

	test('asks which one when a title names more than one', async () => {
		await asAdmin('company_record_add', { category: 'product', title: 'Design award 2026' });

		const asked = await asSample('company_record_list', { query: 'Design award' });
		const refused = await asAdmin('company_record_update', { recordHint: 'Design award', detail: 'Which one?' });

		expect(resultOf(asked).count).toBe(2);
		expect(refused.status).toBe(409);
		expect(errorOf(refused)).toContain('more than one');
		expect(candidatesOf(refused).length).toBe(2);
	});

	test('says what a delete would take before it takes it', async () => {
		const previewed = await previewToolOverTheRecord(
			admin,
			client,
			adminID,
			'company_record_delete',
			{ recordHint: 'Pre-seed round closed' },
			now
		);

		expect(previewed.status).toBe(200);
		expect((previewed.body as { target: { id: string; inputField: string } }).target.id).toBe(seedRecordID);
		expect((previewed.body as { target: { inputField: string } }).target.inputField).toBe('recordHint');
	});

	test('is an administrator to delete, and answers with what it took', async () => {
		const refused = await asSample('company_record_delete', { recordHint: seedRecordID });
		const deleted = await asAdmin('company_record_delete', { recordHint: seedRecordID });
		const left = await asSample('company_record_list');

		expect(refused.status).toBe(403);
		expect(deleted.status).toBe(200);
		expect(resultOf(deleted).title).toBe('Pre-seed round closed');
		expect(resultOf(left).count).toBe(2);
	});
});

describe('the category data room', () => {
	test('starts with the default template and assigns live scopes through a custom role', async () => {
		const initial = await asAdmin('company_dataroom_get');
		expect(resultOf(initial).canManage).toBe(true);
		expect(resultOf(initial).categories).toHaveLength(48);
		const category = await asAdmin('company_dataroom_category_update', {
			code: 'FZ', parent: 'F', slug: 'custom', name: 'Custom finance', nameKO: '추가 재무', description: 'Company-specific finance records.'
		});
		expect(category.status).toBe(200);
		const role = await asAdmin('company_dataroom_role_update', {
			code: 'room-test', name: 'Sample finance reader', nameKO: '', readableCategories: ['F', 'FS']
		});
		expect(role.status).toBe(200);
		const filed = await asAdmin('company_document_register', {
			documentType: 'report', categoryCode: 'FZ', title: 'Sample categorized statement', summary: 'A sample financial record.'
		});
		expect(filed.status).toBe(200);
		expect(resultOf(await asSample('company_document_list', { categoryCode: 'F' })).count).toBe(0);
		const shared = await asAdmin('company_dataroom_share_add', { roleCode: 'room-test', audience: 'member', memberID: sampleID });
		expect(shared.status).toBe(200);
		const shareID = resultOf(shared).shareID;
		expect(typeof shareID).toBe('string');
		expect(resultOf(await asSample('company_document_list', { categoryCode: 'F' })).count).toBe(1);
		const revoked = await asAdmin('company_dataroom_share_delete', { shareID });
		expect(revoked.status).toBe(200);
		expect(resultOf(await asSample('company_document_list', { categoryCode: 'F' })).count).toBe(0);
	});

	test('a colleague cannot expand their own role', async () => {
		const refused = await asSample('company_dataroom_role_update', {
			code: 'employee', name: 'Employee', nameKO: '', readableCategories: ['F']
		});
		expect(refused.status).toBe(403);
	});
});

describe('the document ledger', () => {
	let quoteID = '';

	test('is registered by whoever asked for the document, with its number and where to save it', async () => {
		const registered = await asSample('company_document_register', {
			documentType: 'quote',
			title: 'ABC Trading onboarding consulting quote',
			clearance: 1,
			counterpart: 'ABC Trading',
			language: 'ko',
			summary: 'A quote for onboarding consulting, 12,000,000 KRW, payable within 30 days of delivery.'
		});

		expect(registered.status).toBe(200);
		expect(resultOf(registered).documentNumber).toBe('Q-2026-001');
		expect(resultOf(registered).kind).toBe('issued');
		expect(resultOf(registered).requesterID).toBe(sampleID);
		expect(resultOf(registered).storageDirectory).toBe('/workspace/circles/member/documents/quote');
		quoteID = resultOf(registered).documentID as string;
	});

	test('hands the next number to the next document of the same type', async () => {
		const second = await asAdmin('company_document_register', {
			documentType: 'quote',
			title: 'BCD Manufacturing quote',
			clearance: 1,
			counterpart: 'BCD Manufacturing',
			summary: 'A quote for a second engagement.'
		});

		expect(resultOf(second).documentNumber).toBe('Q-2026-002');
	});

	test('gives a received document no number of ours', async () => {
		const received = await asSample('company_document_register', {
			kind: 'received',
			documentType: 'award-certificate',
			title: 'Excellence award certificate',
			clearance: 1,
			counterpart: 'The Ministry',
			summary: 'The certificate naming the reason the award was given.'
		});

		expect(resultOf(received).documentNumber).toBeNull();
		expect(resultOf(received).kind).toBe('received');
	});

	test('is listed newest first and filtered by type and counterpart', async () => {
		const quotes = await asSample('company_document_list', { type: 'quote' });
		const counterpart = await asSample('company_document_list', { counterpart: 'abc trading' });

		expect(resultOf(quotes).count).toBe(2);
		expect(resultOf(counterpart).count).toBe(1);
		expect((resultOf(counterpart).documents as { documentID: string }[])[0].documentID).toBe(quoteID);
	});

	test('answers a question with the documents nearest to it', async () => {
		const found = await asSample('company_document_search', {
			query: 'ABC Trading onboarding',
			limit: 3
		});

		expect(found.status).toBe(200);
		expect(resultOf(found).count).toBeGreaterThan(0);
		expect((resultOf(found).documents as { documentID: string }[])[0].documentID).toBe(quoteID);
	});

	test('follows a file that was saved under a path, named by its document number', async () => {
		const written = await asSample('company_document_update', {
			documentHint: 'Q-2026-001',
			filePath: 'shared/documents/quote/abc-trading.md'
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).documentID).toBe(quoteID);
		expect(resultOf(written).filePath).toBe('shared/documents/quote/abc-trading.md');
		expect(resultOf(written).title).toBe('ABC Trading onboarding consulting quote');
	});

	test('says so when the document nobody registered is named', async () => {
		const refused = await asSample('company_document_update', {
			documentHint: 'Q-2026-999',
			title: 'A document that is not there'
		});

		expect(refused.status).toBe(409);
		expect((refused.body as { errorCode?: string }).errorCode).toBeDefined();
	});
});

describe('the data room', () => {
	const digest = 'a'.repeat(64);
	let statementID = '';

	test('refuses a member filing above their own clearance', async () => {
		const refused = await asSample('company_document_register', {
			kind: 'internal',
			documentType: 'financial-statement',
			title: '2025 financial statement',
			summary: 'The audited statement for 2025.',
			domain: '03-finance',
			clearance: 2
		});

		expect(refused.status).toBe(403);
	});

	test('files a document at its domain clearance with the frontmatter the standard names', async () => {
		const registered = await asAdmin('company_document_register', {
			kind: 'internal',
			documentType: 'financial-statement',
			title: '2025 financial statement',
			summary: 'The audited statement for 2025.',
			domain: '03-finance',
			clearance: 2,
			date: '2026-03-31',
			period: '2025',
			status: 'current',
			sha256: digest,
			tags: ['audit', 'annual']
		});

		expect(registered.status).toBe(200);
		expect(resultOf(registered).clearance).toBe(2);
		expect(resultOf(registered).domain).toBe('03-finance');
		expect(resultOf(registered).tags).toEqual(['audit', 'annual']);
		expect(resultOf(registered).published).toBeNull();
		statementID = resultOf(registered).documentID as string;
	});

	test('does not exist for a member below its clearance', async () => {
		const forSample = await asSample('company_document_list', { domain: '03-finance' });
		const forAdmin = await asAdmin('company_document_list', { domain: '03-finance', clearance: 2 });

		expect(resultOf(forSample).count).toBe(0);
		expect(resultOf(forAdmin).count).toBe(1);
	});

	test('is superseded by a document in the same domain, never overwritten', async () => {
		const restated = await asAdmin('company_document_register', {
			kind: 'internal',
			documentType: 'financial-statement',
			title: '2025 financial statement, restated',
			summary: 'The 2025 statement restated after the audit adjustment.',
			domain: '03-finance',
			clearance: 2,
			supersedesHint: '2025 financial statement'
		});

		expect(restated.status).toBe(200);
		expect(resultOf(restated).supersedes).toBe(statementID);
	});

	test('signs an upload at the requester clearance and refuses one above it', async () => {
		const allowed = await asSample('company_document_upload', { clearance: 1, sha256: digest });
		const refused = await asSample('company_document_upload', { clearance: 2, sha256: digest });

		expect(allowed.status).toBe(200);
		expect(resultOf(allowed).storagePath).toBe(`${companyID}/dataroom/1/${digest}`);
		expect(String(resultOf(allowed).uploadURL)).toContain('/upload/sign/');
		expect(refused.status).toBeGreaterThanOrEqual(400);
	});

	test('hands the stored file back through a signed download named by the document', async () => {
		const signed = await asSample('company_document_upload', { clearance: 1, sha256: digest, fileName: 'text.md' });
		const put = await fetch(String(resultOf(signed).uploadURL), {
			method: 'PUT',
			headers: { 'content-type': 'text/markdown' },
			body: '# the derived text'
		});
		expect(put.ok).toBe(true);

		const registered = await asSample('company_document_register', {
			kind: 'internal',
			documentType: 'product-brief',
			title: 'internkim product brief',
			summary: 'What internkim is, for a member who asks.',
			domain: '08-product',
			clearance: 1,
			sha256: digest,
			storagePath: `${companyID}/dataroom/1/${digest}`
		});
		const download = await asSample('company_document_download', {
			documentHint: resultOf(registered).documentID,
			fileName: 'text.md'
		});

		expect(download.status).toBe(200);
		expect(resultOf(download).storagePath).toBe(`${companyID}/dataroom/1/${digest}/text.md`);
		const fetched = await fetch(String(resultOf(download).downloadURL));
		expect(await fetched.text()).toBe('# the derived text');
	});

	test('says so when the named document keeps no file', async () => {
		const refused = await asSample('company_document_download', { documentHint: 'Q-2026-001' });

		expect(refused.status).toBe(404);
	});

	test('reads the domain once an administrator raises the member clearance', async () => {
		const raised = await asAdmin('person_update', { personHint: '이샘플', clearance: 2 });
		expect(raised.status).toBe(200);
		expect(resultOf(raised).clearance).toBe(2);

		const forSample = await asSample('company_document_list', { domain: '03-finance' });
		expect(resultOf(forSample).count).toBe(2);
	});
});

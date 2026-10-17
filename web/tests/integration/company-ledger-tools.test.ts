import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { z } from 'zod';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';
import { circleListResultSchema } from '../../src/lib/data-room/schemas';

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
let colleagueID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let colleague: ReturnType<typeof asMember>;
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

	colleagueID = await addMember(client, companyID, `${slug}-colleague@example.test`);
	colleague = await signedInMember(colleagueID, `${slug}-colleague@example.test`);
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

async function asColleague(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(colleague, client, colleagueID, name, input, now, leavesTaskLabelsUndecided));
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
	'company_document_recall',
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
			amount: { amount: 1_200_000_000, currency: 'KRW' },
			usdEquivalent: 870_000
		});

		expect(refused.status).toBe(403);
		expect(errorOf(refused)).toContain('administrator');
	});

	test('records money with the USD equivalent a reader can compare', async () => {
		const written = await asAdmin('company_metric_record', {
			metric: 'annualRevenue',
			year: 2025,
			amount: { amount: 1_200_000_000, currency: 'KRW' },
			usdEquivalent: 870_000
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).amount).toEqual({ amount: 1_200_000_000, currency: 'KRW' });
		expect(resultOf(written).usdEquivalent).toBe(870_000);
		expect(resultOf(written).value).toBeNull();
		expect(resultOf(written).quarter).toBe(0);
	});

	test('takes money in any ISO currency', async () => {
		const written = await asAdmin('company_metric_record', {
			metric: 'vietnamRevenue',
			year: 2023,
			amount: { amount: 25_000_000_000, currency: 'VND' },
			usdEquivalent: 1_000_000
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).amount).toEqual({ amount: 25_000_000_000, currency: 'VND' });
	});

	test('refuses money whose USD equivalent nobody gave', async () => {
		const refused = await asAdmin('company_metric_record', {
			metric: 'operatingProfit',
			year: 2025,
			amount: { amount: 1_000, currency: 'KRW' }
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
			amount: { amount: 1_300_000_000, currency: 'USD' }
		});
		const listed = await asAdmin('company_metric_list', { metric: 'annualRevenue' });

		expect(listed.status).toBe(200);
		expect(resultOf(listed).count).toBe(1);
		expect(z.array(z.object({ amount: z.object({ amount: z.number() }) })).parse(resultOf(listed).metrics)[0].amount.amount).toBe(1_300_000_000);
	});

	test('is read by anybody who works here, and filtered by year', async () => {
		await asAdmin('company_metric_record', { metric: 'mau', year: 2024, month: 12, value: 9_000, unit: 'people' });

		const everything = await asSample('company_metric_list');
		const recent = await asSample('company_metric_list', { fromYear: 2025 });

		expect(resultOf(everything).count).toBe(3);
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
			amount: { amount: 2_000_000, currency: 'USD' },
			attributes: '{"round": "Seed", "investors": "ABC Ventures"}'
		});

		expect(added.status).toBe(200);
		expect(resultOf(added).amount).toEqual({ amount: 2_000_000, currency: 'USD' });
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
	test('places people in circles and creates and revokes code-protected links', async () => {
		const assigned = await asAdmin('circle_member_update', {
			memberID: sampleID, circleIDs: ['finance']
		});
		expect(assigned.status).toBe(200);
		const created = await asSample('dataroom_link_add', {
			circleID: 'finance', label: 'Sample finance review'
		});
		expect(created.status).toBe(200);
		const linkID = resultOf(created).linkID;
		expect(resultOf(created).accessCode).toMatch(/^[0-9]{6}$/);
		const listed = await asSample('dataroom_links_get');
		expect(listed.status).toBe(200);
		expect(resultOf(listed).links).toEqual(expect.arrayContaining([expect.objectContaining({ id: linkID })]));
		const revoked = await asSample('dataroom_link_delete', { linkID });
		expect(revoked.status).toBe(200);
		const restored = await asAdmin('circle_member_update', {
			memberID: sampleID, circleIDs: ['member']
		});
		expect(restored.status).toBe(200);
	});
	test('starts with the default template and opens live scopes to the people of a custom circle', async () => {
		const initial = await asAdmin('dataroom_get');
		expect(resultOf(initial).canManage).toBe(true);
		expect(resultOf(initial).categories).toHaveLength(48);
		const category = await asAdmin('dataroom_category_update', {
			code: 'FZ', parent: 'F', slug: 'custom', name: 'Custom finance', nameKO: '추가 재무', description: 'Company-specific finance records.'
		});
		expect(category.status).toBe(200);
		const circle = await asAdmin('circle_update', {
			id: 'room-test', name: 'Sample finance reader', readableCategories: ['F', 'FS']
		});
		expect(circle.status).toBe(200);
		const filed = await asAdmin('company_document_register', {
			documentType: 'report', categoryCode: 'FZ', title: 'Sample categorized statement', summary: 'A sample financial record.'
		});
		expect(filed.status).toBe(200);
		expect(resultOf(await asSample('company_document_list', { categoryCode: 'F' })).count).toBe(0);
		const joined = await asAdmin('circle_member_update', { memberID: sampleID, circleIDs: ['member', 'room-test'] });
		expect(joined.status).toBe(200);
		const { circles } = circleListResultSchema.parse(resultOf(await asSample('circle_list')));
		expect(circles.find((listed) => listed.id === 'room-test'))
			.toEqual({
				id: 'room-test', name: 'Sample finance reader', isDefault: false, readableCategories: ['F'], memberIDs: [sampleID],
				channelName: null, channelID: null
			});
		expect(resultOf(await asSample('company_document_list', { categoryCode: 'F' })).count).toBe(1);
		const left = await asAdmin('circle_member_update', { memberID: sampleID, circleIDs: ['member'] });
		expect(left.status).toBe(200);
		expect(resultOf(await asSample('company_document_list', { categoryCode: 'F' })).count).toBe(0);
	});

	test('gives a circle a channel of its own and deletes it only by its exact name', async () => {
		const refused = await asSample('circle_channel_add', { circleID: 'finance', name: 'Sample finance' });
		expect(refused.status).toBe(403);
		const added = await asAdmin('circle_channel_add', { circleID: 'finance', name: 'Sample finance' });
		expect(added.status).toBe(200);
		const listed = circleListResultSchema.parse(resultOf(await asSample('circle_list'))).circles;
		expect(listed.find((circle) => circle.id === 'finance')).toMatchObject({ channelName: 'Sample finance', channelID: null });
		const twice = await asAdmin('circle_channel_add', { circleID: 'finance', name: 'Another finance' });
		expect(twice.status).toBe(409);
		const misnamed = await asAdmin('circle_channel_delete', { circleID: 'finance', name: 'sample finance' });
		expect(misnamed.status).toBe(422);
		const deleted = await asAdmin('circle_channel_delete', { circleID: 'finance', name: 'Sample finance' });
		expect(deleted.status).toBe(200);
		const after = circleListResultSchema.parse(resultOf(await asSample('circle_list'))).circles;
		expect(after.find((circle) => circle.id === 'finance')).toMatchObject({ channelName: null });
	});

	test('lets an administrator, and nobody else, choose who sees a company field', async () => {
		const refused = await asSample('dataroom_field_update', { field: 'capital', categoryCode: 'GP' });
		expect(refused.status).toBeGreaterThanOrEqual(400);
		const moved = await asAdmin('dataroom_field_update', { field: 'capital', categoryCode: 'GP' });
		expect(moved.status).toBe(200);
		const categories = z.array(z.object({ code: z.string(), fields: z.array(z.string()) }))
			.parse(resultOf(await asAdmin('dataroom_get')).categories);
		expect(categories.filter((category) => category.fields.includes('capital')).map((category) => category.code)).toEqual(['GP']);
		const restored = await asAdmin('dataroom_field_update', { field: 'capital', categoryCode: 'CO' });
		expect(restored.status).toBe(200);
	});

	test('lends what a circle reads to someone outside the company, and takes it back', async () => {
		const shared = await asAdmin('dataroom_share_add', {
			circleID: 'investor', audience: 'email', email: 'circle-guest@example.test'
		});
		expect(shared.status).toBe(200);
		const shareID = resultOf(shared).shareID;
		expect(resultOf(await asAdmin('dataroom_get')).shares).toEqual(expect.arrayContaining([
			expect.objectContaining({ id: shareID, circleID: 'investor', audience: 'email', email: 'circle-guest@example.test' })
		]));
		const revoked = await asAdmin('dataroom_share_delete', { shareID });
		expect(revoked.status).toBe(200);
	});

	test('names a default circle in the company language and never renames it', async () => {
		const { circles } = circleListResultSchema.parse(resultOf(await asAdmin('circle_list')));
		expect(circles.find((circle) => circle.id === 'leadership')).toMatchObject({ isDefault: true });
		const renamed = await asAdmin('circle_update', { id: 'leadership', name: 'Bosses', readableCategories: ['F'] });
		expect(renamed.status).toBe(422);
	});

	test('a colleague cannot widen what their own circle reads', async () => {
		const refused = await asSample('circle_update', {
			id: 'member', readableCategories: ['F']
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
			counterpart: 'ABC Trading',
			language: 'ko',
			summary: 'A quote for onboarding consulting, 12,000,000 KRW, payable within 30 days of delivery.'
		});

		expect(registered.status).toBe(200);
		expect(resultOf(registered).documentNumber).toBe('Q-2026-001');
		expect(resultOf(registered).kind).toBe('issued');
		expect(resultOf(registered).requesterID).toBe(sampleID);
		expect(resultOf(registered).storageDirectory).toBe('/workspace/circles/member/documents/quote');
		expect(Date.parse(String(resultOf(registered).createdAt))).toBeGreaterThan(Date.now() - 60_000);
		expect(resultOf(registered).updatedAt).toBe(resultOf(registered).createdAt);
		expect(resultOf(registered).sizeBytes).toBeNull();
		quoteID = resultOf(registered).documentID as string;
	});

	test('hands the next number to the next document of the same type', async () => {
		const second = await asAdmin('company_document_register', {
			documentType: 'quote',
			title: 'BCD Manufacturing quote',
			categoryCode: 'SS',
			counterpart: 'BCD Manufacturing',
			summary: 'A quote for a second engagement.'
		});

		expect(resultOf(second).documentNumber).toBe('Q-2026-002');
	});

	test('numbers a second colleague\'s document after one it cannot see', async () => {
		const first = await asSample('company_document_register', { documentType: 'invoice', title: 'Sample invoice one' });
		const second = await asColleague('company_document_register', { documentType: 'invoice', title: 'Sample invoice two' });

		expect(resultOf(first).documentNumber).toBe('INV-2026-001');
		expect(second.status).toBe(200);
		expect(resultOf(second).documentNumber).toBe('INV-2026-002');
		expect(resultOf(await asColleague('company_document_list', { type: 'invoice' })).count).toBe(1);
	});

	test('gives a received document no number of ours', async () => {
		const received = await asSample('company_document_register', {
			kind: 'received',
			documentType: 'award-certificate',
			title: 'Excellence award certificate',
			categoryCode: 'GP',
			counterpart: 'The Ministry',
			summary: 'The certificate naming the reason the award was given.'
		});

		expect(resultOf(received).documentNumber).toBeNull();
		expect(resultOf(received).kind).toBe('received');
	});

	test('is listed newest first and filtered by type and counterpart', async () => {
		const quotes = await asAdmin('company_document_list', { type: 'quote' });
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

	test('recalls the documents a request names, one sentence each, and none for a request naming nothing', async () => {
		const recalled = await asSample('company_document_recall', {
			query: 'What did we promise ABC Trading for the onboarding, and by when?',
			limit: 3
		});

		expect(recalled.status).toBe(200);
		const facts = resultOf(recalled).facts as { id: string; content: string }[];
		expect(facts.find((fact) => fact.id === quoteID)?.content).toContain('ABC Trading onboarding consulting quote');

		const unrelated = await asSample('company_document_recall', {
			query: 'Book the design review for three tomorrow afternoon',
			limit: 3
		});
		expect(resultOf(unrelated).facts).toEqual([]);
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
	let briefID = '';

	test('refuses a member filing in a category their circles do not read', async () => {
		const refused = await asSample('company_document_register', {
			kind: 'internal',
			documentType: 'financial-statement',
			title: '2025 financial statement',
			summary: 'The audited statement for 2025.',
			categoryCode: 'FS'
		});

		expect(refused.status).toBe(403);
	});

	test('files a document in a category with the frontmatter the standard names', async () => {
		const registered = await asAdmin('company_document_register', {
			kind: 'internal',
			documentType: 'financial-statement',
			title: '2025 financial statement',
			summary: 'The audited statement for 2025.',
			categoryCode: 'FS',
			date: '2026-03-31',
			period: '2025',
			status: 'current',
			sha256: digest,
			tags: ['audit', 'annual']
		});

		expect(registered.status).toBe(200);
		expect(resultOf(registered).categoryCode).toBe('FS');
		expect(resultOf(registered).tags).toEqual(['audit', 'annual']);
		expect(resultOf(registered).published).toBeNull();
		statementID = resultOf(registered).documentID as string;
	});

	test('does not exist for a member whose circles do not read its category', async () => {
		const forSample = await asSample('company_document_list', { categoryCode: 'FS' });
		const forAdmin = await asAdmin('company_document_list', { categoryCode: 'FS' });

		expect(resultOf(forSample).count).toBe(0);
		expect(resultOf(forAdmin).count).toBe(1);
	});

	test('is superseded by a document in the same category, never overwritten', async () => {
		const restated = await asAdmin('company_document_register', {
			kind: 'internal',
			documentType: 'financial-statement',
			title: '2025 financial statement, restated',
			summary: 'The 2025 statement restated after the audit adjustment.',
			categoryCode: 'FS',
			supersedesHint: '2025 financial statement'
		});

		expect(restated.status).toBe(200);
		expect(resultOf(restated).supersedes).toBe(statementID);
	});

	test('names the original by its category, its name and its document', async () => {
		const registered = await asSample('company_document_register', {
			kind: 'internal',
			documentType: 'product-brief',
			title: 'internkim product brief',
			summary: 'What internkim is, for a member who asks.',
			categoryCode: 'PO',
			sha256: digest
		});
		briefID = resultOf(registered).documentID as string;

		const signed = await asSample('company_document_upload', { documentHint: briefID, originalFileName: 'InternKim Brief.pdf' });
		expect(signed.status).toBe(200);
		expect(resultOf(signed).storagePath).toBe(`${companyID}/dataroom/P/PO/internkim-brief.${briefID}.pdf`);
		const put = await fetch(String(resultOf(signed).uploadURL), {
			method: 'PUT',
			headers: { 'content-type': 'application/pdf' },
			body: '%PDF the brief'
		});
		expect(put.ok).toBe(true);
	});

	test('refuses an upload for a document the member may not change', async () => {
		const refused = await asSample('company_document_upload', { documentHint: statementID, originalFileName: 'statement.pdf' });

		expect(refused.status).toBeGreaterThanOrEqual(400);
	});

	test('hands a derived file back from beside its original', async () => {
		const signed = await asSample('company_document_upload', { documentHint: briefID, fileName: 'content.txt' });
		expect(resultOf(signed).storagePath).toBe(`${companyID}/dataroom/P/PO/internkim-brief.${briefID}.content.txt`);
		const put = await fetch(String(resultOf(signed).uploadURL), {
			method: 'PUT',
			headers: { 'content-type': 'text/plain' },
			body: 'the derived text'
		});
		expect(put.ok).toBe(true);

		const download = await asSample('company_document_download', { documentHint: briefID, fileName: 'content.txt' });
		expect(download.status).toBe(200);
		const fetched = await fetch(String(resultOf(download).downloadURL));
		expect(await fetched.text()).toBe('the derived text');
	});

	test('keeps the files where they are when a member may not reclassify', async () => {
		const refused = await asSample('company_document_update', { documentHint: briefID, categoryCode: 'GP' });
		expect(refused.status).toBe(403);

		const download = await asSample('company_document_download', { documentHint: briefID, fileName: 'content.txt' });
		expect(resultOf(download).storagePath).toBe(`${companyID}/dataroom/P/PO/internkim-brief.${briefID}.content.txt`);
		expect(await (await fetch(String(resultOf(download).downloadURL))).text()).toBe('the derived text');
	});

	test('moves the files with the document when an administrator reclassifies it', async () => {
		const moved = await asAdmin('company_document_update', { documentHint: briefID, categoryCode: 'GP' });
		expect(moved.status).toBe(200);
		expect(resultOf(moved).storagePath).toBe(`${companyID}/dataroom/G/GP/internkim-brief.${briefID}.pdf`);

		const download = await asSample('company_document_download', { documentHint: briefID, fileName: 'content.txt' });
		expect(resultOf(download).storagePath).toBe(`${companyID}/dataroom/G/GP/internkim-brief.${briefID}.content.txt`);
		expect(await (await fetch(String(resultOf(download).downloadURL))).text()).toBe('the derived text');
	});

	test('reclassifies a document whose original was named but never uploaded', async () => {
		const registered = await asSample('company_document_register', {
			kind: 'internal',
			documentType: 'memo',
			title: 'unsent memo',
			summary: 'A memo whose file never arrived.'
		});
		const memoID = resultOf(registered).documentID as string;
		await asSample('company_document_upload', { documentHint: memoID, originalFileName: 'memo.txt' });

		const moved = await asAdmin('company_document_update', { documentHint: memoID, categoryCode: 'GC' });
		expect(moved.status).toBe(200);
		expect(resultOf(moved).storagePath).toBe(`${companyID}/dataroom/G/GC/memo.${memoID}.txt`);
	});

	test('says so when the named document keeps no file', async () => {
		const refused = await asSample('company_document_download', { documentHint: 'Q-2026-001' });

		expect(refused.status).toBe(404);
	});
});

describe('the round being raised and who holds the company', () => {
	let seriesAID = '';

	test('a round being raised is a funding record in progress until it closes', async () => {
		const raising = await asAdmin('company_record_add', {
			category: 'funding',
			date: '2026-12-18',
			title: 'Series A',
			amount: { amount: 20_000_000_000, currency: 'KRW' },
			status: 'in_progress'
		});
		expect(raising.status).toBe(200);
		expect(resultOf(raising).status).toBe('in_progress');
		seriesAID = resultOf(raising).recordID as string;

		const closed = await asAdmin('company_record_update', { recordHint: seriesAID, status: 'completed' });
		expect(resultOf(closed).status).toBe('completed');
	});

	test('only an administrator keeps the ledger, and says so instead of showing it empty', async () => {
		const refusedWrite = await asSample('company_equity_add', {
			effectiveDate: '2021-03-15', holder: 'Sample Kim', holderKind: 'founder', shares: 600_000
		});
		const refusedRead = await asSample('company_equity_list');

		expect(refusedWrite.status).toBe(403);
		expect(refusedRead.status).toBe(403);
		expect(errorOf(refusedRead)).toContain('administrator');
	});

	test('sums its lines into holdings, as of any day', async () => {
		await asAdmin('company_equity_add', { effectiveDate: '2021-03-15', holder: 'Sample Kim', holderKind: 'founder', shares: 600_000 });
		await asAdmin('company_equity_add', { effectiveDate: '2022-03-01', holder: 'Option pool', holderKind: 'option_pool', securityClass: 'options', shares: 100_000 });
		const issued = await asAdmin('company_equity_add', {
			effectiveDate: '2026-12-18', holder: 'Sample Ventures Fund I', holderKind: 'investor', securityClass: 'RCPS',
			shares: 300_000, fundingRoundHint: 'Series A'
		});
		expect(resultOf(issued).fundingRecordID).toBe(seriesAID);

		const now = resultOf(await asAdmin('company_equity_list', { asOfDate: '2026-12-31' }));
		expect(now.totalShares).toBe(1_000_000);
		expect((now.holdings as { holder: string; percent: number }[]).map((holding) => [holding.holder, holding.percent]))
			.toEqual([['Sample Kim', 60], ['Sample Ventures Fund I', 30], ['Option pool', 10]]);

		const beforeTheRound = resultOf(await asAdmin('company_equity_list', { asOfDate: '2026-12-17' }));
		expect(beforeTheRound.totalShares).toBe(700_000);
	});

	test('refuses a line that moves no shares', async () => {
		const refused = await asAdmin('company_equity_add', { effectiveDate: '2026-01-01', holder: 'Sample Kim', holderKind: 'founder', shares: 0 });

		expect(refused.status).toBe(400);
	});

	test('deletes a wrong line by its id and nothing else', async () => {
		const wrong = await asAdmin('company_equity_add', { effectiveDate: '2026-02-01', holder: 'Typo Holder', holderKind: 'other', shares: 5 });
		const wrongID = resultOf(wrong).entryID as string;

		const deleted = await asAdmin('company_equity_delete', { entryHint: wrongID });
		expect(resultOf(deleted).entryID).toBe(wrongID);

		const again = await asAdmin('company_equity_delete', { entryHint: wrongID });
		expect(again.status).toBe(409);
		expect((again.body as { errorCode?: string }).errorCode).toBe('company_equity_entry_not_found');
		expect(resultOf(await asAdmin('company_equity_list', { asOfDate: '2026-12-31' })).totalShares).toBe(1_000_000);
	});
});

import { expect, test, type Locator, type Page } from '@playwright/test';
import { signInToAttendance } from './attendance-central-test-utils';
import { centralPlaneAdminClient, exampleCompanyID } from './central-test-utils';
import {
	type Locale,
	measureLeaveBalanceSummary,
	switchLocale,
	usedLabelOf
} from './attendance-leave-summary-helpers';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

const locales: Locale[] = ['ko', 'en'];

let seededLeaveDays: number | null = null;

test.beforeAll(async () => {
	const stored = await centralPlaneAdminClient()
		.from('leave')
		.select('days, member!inner(company_id)')
		.eq('member.company_id', exampleCompanyID)
		.is('status', null)
		.limit(1)
		.maybeSingle();
	if (stored.error) throw new Error(`Failed to read the company leave grant: ${stored.error.message}`);
	seededLeaveDays = stored.data ? Number(stored.data.days) : null;
});

test.afterAll(async () => {
	await setLeaveDays(seededLeaveDays);
});

// A balance is the grants that stand, so turning one off is removing them and
// turning it back on is writing them again.
async function setLeaveDays(leaveDays: number | null): Promise<void> {
	const admin = centralPlaneAdminClient();
	const people = await admin.from('member').select('id').eq('company_id', exampleCompanyID);
	if (people.error) throw new Error(`Failed to read the company members: ${people.error.message}`);
	const memberIDs = people.data.map((person) => person.id as string);

	const removed = await admin.from('leave').delete().is('status', null).in('member_id', memberIDs);
	if (removed.error) throw new Error(`Failed to clear the company leave grant: ${removed.error.message}`);
	if (leaveDays === null) return;

	const written = await admin.from('leave').insert(
		memberIDs.map((memberID) => ({
			member_id: memberID,
			kind: 'annual',
			is_paid: true,
			is_deducted: false,
			days: leaveDays,
			granted_on: '1970-01-01',
			origin: 'manual'
		}))
	);
	if (written.error) throw new Error(`Failed to write the company leave grant: ${written.error.message}`);
}

async function summaryCard(page: Page): Promise<Locator> {
	const card = page.locator('[data-testid="leave-balance-summary"]:visible').first();
	await card.waitFor({ state: 'visible', timeout: 30000 });
	return card;
}

async function columnsIn(page: Page, locale: Locale) {
	await switchLocale(page, locale);
	const columns = await measureLeaveBalanceSummary(await summaryCard(page));
	expect(columns[0]?.label, `the card did not render in ${locale}`).toBe(usedLabelOf[locale]);
	return columns;
}

test('every leave summary column starts its label and its value on one edge', async ({ page }) => {
	await setLeaveDays(seededLeaveDays);
	await signInToAttendance(page);

	for (const locale of locales) {
		const columns = await columnsIn(page, locale);
		expect(columns, locale).toHaveLength(3);
		for (const column of columns) {
			expect(column.labelLeft, `${locale} ${column.label}`).toBe(column.valueLeft);
		}
	}
});

test('an unlimited balance stays inside the column that holds it', async ({ page }) => {
	await setLeaveDays(null);
	await signInToAttendance(page);

	for (const locale of locales) {
		const columns = await columnsIn(page, locale);
		const available = columns[columns.length - 1];
		expect(available.valueTextWidth, `${locale} ${available.value}`).toBeLessThanOrEqual(
			available.columnWidth
		);
		for (const column of columns) {
			expect(column.labelTextWidth, `${locale} ${column.label}`).toBeLessThanOrEqual(column.columnWidth);
		}
	}
});

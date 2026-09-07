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
		.from('company')
		.select('leave_days')
		.eq('id', exampleCompanyID)
		.single();
	if (stored.error) throw new Error(`Failed to read the company leave grant: ${stored.error.message}`);
	seededLeaveDays = stored.data.leave_days as number | null;
});

test.afterAll(async () => {
	await setLeaveDays(seededLeaveDays);
});

async function setLeaveDays(leaveDays: number | null): Promise<void> {
	const written = await centralPlaneAdminClient()
		.from('company')
		.update({ leave_days: leaveDays })
		.eq('id', exampleCompanyID);
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

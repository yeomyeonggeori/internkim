import { expect, test, type Page } from '@playwright/test';
import { signInToAttendance } from './attendance-central-test-utils';
import { centralPlaneAdminClient, exampleCompanyID } from './central-test-utils';
import {
	type Locale,
	switchLocale
} from './attendance-leave-summary-helpers';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

const locales: Locale[] = ['ko', 'en'];

let seededLeaveDays: number | null = null;
let seededRules: Record<string, unknown> = {};

test.beforeAll(async () => {
	const company = await centralPlaneAdminClient().from('company').select('rules').eq('id', exampleCompanyID).single();
	if (company.error) throw new Error(company.error.message);
	seededRules = company.data.rules;
	const stored = await centralPlaneAdminClient()
		.from('leave')
		.select('days, member!inner(company_id)')
		.eq('member.company_id', exampleCompanyID)
		.gte('days', 0)
		.limit(1)
		.maybeSingle();
	if (stored.error) throw new Error(`Failed to read the company leave grant: ${stored.error.message}`);
	seededLeaveDays = stored.data ? Number(stored.data.days) : null;
});

test.afterAll(async () => {
	await setLeaveDays(seededLeaveDays);
	const restored = await centralPlaneAdminClient().from('company').update({rules:seededRules}).eq('id', exampleCompanyID);
	if (restored.error) throw new Error(restored.error.message);
});

// Unlimited policy disables automatic accrual; removing grants alone would
// cause managed annual leave to be granted again on the next read.
async function setLeaveDays(leaveDays: number | null): Promise<void> {
	const admin = centralPlaneAdminClient();
	const policy = seededRules.attendanceLeavePolicy as Record<string, unknown>;
	const configured = await admin.from('company').update({rules:{...seededRules, attendanceLeavePolicy:{...policy,balanceTrackingMode:leaveDays===null?'unlimited':'managed'}}}).eq('id',exampleCompanyID);
	if (configured.error) throw new Error(configured.error.message);
	const people = await admin.from('member').select('id').eq('company_id', exampleCompanyID);
	if (people.error) throw new Error(`Failed to read the company members: ${people.error.message}`);
	const memberIDs = people.data.map((person) => person.id as string);

	const removed = await admin.from('leave').delete().gte('days', 0).in('member_id', memberIDs);
	if (removed.error) throw new Error(`Failed to clear the company leave grant: ${removed.error.message}`);
	if (leaveDays === null) return;

	const written = await admin.from('leave').insert(
		memberIDs.map((memberID) => ({
			member_id: memberID,
			kind: 'annual',
			is_paid: true,
			is_deducted: false,
			days: leaveDays,
			status: 'approved',
			granted_on: '1970-01-01',
			origin: 'manual'
		}))
	);
	if (written.error) throw new Error(`Failed to write the company leave grant: ${written.error.message}`);
}

async function balancesIn(page: Page, locale: Locale, unlimited: boolean) {
	await switchLocale(page, locale);
	await page.getByRole('button', {name:locale === 'ko' ? '내 휴가' : 'My leave', exact:true}).click();
	const balances = page.getByTestId('leave-type-balances');
	const cards = balances.locator(':scope > div > div');
	await expect(cards.first()).toBeVisible();
	const label = unlimited ? (locale === 'ko' ? '사용' : 'Used') : (locale === 'ko' ? '남음' : 'Available');
	await expect(cards.first()).toContainText(label);
	const value = cards.first().locator('span.text-2xl');
	await expect.poll(async () => Number.parseFloat(await value.innerText())).toBe(unlimited ? 0 : seededLeaveDays);
	for (const card of await cards.all()) {
		const contained = await card.evaluate(element => {
			const box = element.getBoundingClientRect();
			return [...element.querySelectorAll('p')].every(node => {
				const range = document.createRange(); range.selectNodeContents(node);
				const text = range.getBoundingClientRect();
				return text.left >= box.left && text.right <= box.right;
			});
		});
		expect(contained, locale).toBe(true);
	}
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

test('per-type remaining balances keep their labels and actual grants readable on mobile', async ({ page }) => {
	await setLeaveDays(seededLeaveDays);
	await page.setViewportSize({width:320,height:844});
	await signInToAttendance(page);
	for (const locale of locales) {
		await balancesIn(page, locale, false);
	}
});

test('unlimited leave shows actual used balances without overflowing on mobile', async ({ page }) => {
	await setLeaveDays(null);
	await page.setViewportSize({width:320,height:844});
	await signInToAttendance(page);
	for (const locale of locales) {
		await balancesIn(page, locale, true);
	}
});

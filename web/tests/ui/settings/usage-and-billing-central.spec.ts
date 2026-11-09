import { expect, test, type Page } from '@playwright/test';
import {
	centralPlaneAdminClient,
	exampleCompanyID,
	exampleCompanySlug,
	member1ID,
	member3Email
} from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const settingsPath = `/${exampleCompanySlug}/settings`;
const subscriptionID = `sub_ui_${exampleCompanyID}`;
const customerID = `cus_ui_${exampleCompanyID}`;

async function openAdminTab(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, settingsPath);
	await page.getByRole('tab', { name: '관리자' }).click();
	await expect(page.getByRole('heading', { name: '사용량 및 결제' })).toBeVisible({ timeout: 20000 });
}

async function subscribeTheCompany(): Promise<void> {
	const client = centralPlaneAdminClient();
	const paid = await client.rpc('company_subscription_invoice_paid', {
		target_company: exampleCompanyID,
		target_event: `evt_ui_${exampleCompanyID}_${Date.now()}`,
		target_customer: customerID,
		target_subscription: subscriptionID,
		granted_credits: 40000,
		opens_period: true,
		period_ends_at: new Date(Date.now() + 20 * 86_400_000).toISOString()
	});
	if (paid.error) throw new Error(paid.error.message);
	const changed = await client.rpc('company_subscription_changed', {
		target_company: exampleCompanyID,
		target_customer: customerID,
		target_subscription: subscriptionID,
		target_status: 'active',
		status_at: new Date().toISOString()
	});
	if (changed.error) throw new Error(changed.error.message);
	const used = await client.from('company_credit_entry').insert({
		company_id: exampleCompanyID,
		member_id: member1ID,
		credit_kind: 'subscription',
		entry_kind: 'usage',
		credits: -8400
	});
	if (used.error) throw new Error(used.error.message);
}

async function unsubscribeTheCompany(): Promise<void> {
	const client = centralPlaneAdminClient();
	const held = await client.from('company').select('rules').eq('id', exampleCompanyID).single();
	if (held.error) throw new Error(held.error.message);
	const rules = { ...(held.data.rules ?? {}) };
	delete rules.creditPolicy;
	const company = await client.from('company').update({
		stripe_customer_id: null,
		stripe_subscription_id: null,
		subscription_status: null,
		subscription_ends_at: null,
		subscription_credits: 0,
		permanent_credits: 0,
		rules
	}).eq('id', exampleCompanyID);
	if (company.error) throw new Error(company.error.message);
	const members = await client.from('member').update({ credits_used: 0 }).eq('company_id', exampleCompanyID);
	if (members.error) throw new Error(members.error.message);
}

test.afterAll(unsubscribeTheCompany);

test('an administrator of an unsubscribed company is offered the subscription', async ({ page }) => {
	await openAdminTab(page);
	await expect(page.getByRole('button', { name: '구독 시작' })).toBeVisible();
	await expect(page.getByText('회사 크레딧', { exact: true })).toHaveCount(0);
});

test('an administrator of a subscribed company sees the credits and sets one limit for everybody', async ({ page }) => {
	await subscribeTheCompany();
	await openAdminTab(page);
	await expect(page.getByText('이용 중', { exact: true })).toBeVisible();
	await expect(page.getByText('31,600 남음')).toBeVisible();
	await expect(page.getByText('8,400 / 10,000')).toBeVisible();

	const memberUsage = page.locator('[data-slot="card"]').filter({ has: page.getByText('팀원별 사용량', { exact: true }) });
	await memberUsage.getByLabel('1인 한도').fill('12000');
	await memberUsage.getByRole('button', { name: '저장', exact: true }).click();
	await expect(page.getByText('8,400 / 12,000')).toBeVisible();

	await page.reload();
	await page.getByRole('tab', { name: '관리자' }).click();
	await expect(page.getByLabel('1인 한도')).toHaveValue('12000');
});

test('a member sees their own usage as a share and nothing of the billing', async ({ page }) => {
	await signInToTheCentralPlane(page, settingsPath, member3Email);
	await expect(page.getByText('내 사용량', { exact: true })).toBeVisible({ timeout: 20000 });
	await expect(page.getByText('0% 사용')).toBeVisible();
	await expect(page.getByRole('heading', { name: '사용량 및 결제' })).toHaveCount(0);
});

test('an administrator whose card was declined is not offered a second subscription', async ({ page }) => {
	const declined = await centralPlaneAdminClient().rpc('company_subscription_changed', {
		target_company: exampleCompanyID,
		target_customer: customerID,
		target_subscription: subscriptionID,
		target_status: 'past_due',
		status_at: new Date().toISOString()
	});
	if (declined.error) throw new Error(declined.error.message);
	await openAdminTab(page);
	await expect(page.getByText('결제 확인 필요', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: '구독 시작' })).toHaveCount(0);
});

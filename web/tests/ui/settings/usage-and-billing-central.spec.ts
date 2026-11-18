import { expect, test, type Page } from '@playwright/test';
import {
	centralPlaneAdminClient,
	exampleCompanyID,
	exampleCompanySlug,
	member1ID,
	member3Email
} from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { customerID, subscribeTheCompany, subscriptionID, unsubscribeTheCompany } from './company-subscription';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const settingsPath = `/${exampleCompanySlug}/settings`;
const computerKeyHash = `billing-ui-${exampleCompanyID}`;
const keyPaysUntilNextStart = '회사 컴퓨터가 아직 회사 OpenRouter 키로 AI 사용료를 내고 있습니다. 서비스가 다음에 시작하면 크레딧으로 바뀝니다.';
const keyPaysNow = '지금은 회사 OpenRouter 키가 AI 사용료를 냅니다.';

async function computerStartedWithItsKey(): Promise<void> {
	const { error } = await centralPlaneAdminClient().from('agent').upsert(
		{
			company_id: exampleCompanyID,
			name: 'company-computer',
			api_key_hash: computerKeyHash,
			revoked_at: null,
			holds_model_key: true,
			uses_own_model_key_at_start: true,
			model_key_reported_at: new Date().toISOString()
		},
		{ onConflict: 'company_id,name' }
	);
	if (error) throw new Error(error.message);
}

async function forgetTheComputer(): Promise<void> {
	const { error } = await centralPlaneAdminClient().from('agent').delete().eq('api_key_hash', computerKeyHash);
	if (error) throw new Error(error.message);
}

async function companyKeyPays(usesOwnModelKey: boolean): Promise<void> {
	const { error } = await centralPlaneAdminClient().from('company').update({ uses_own_model_key: usesOwnModelKey }).eq('id', exampleCompanyID);
	if (error) throw new Error(error.message);
}

async function openBilling(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, `${settingsPath}/billing`);
	await expect(page.getByRole('heading', { name: '사용량 및 결제' })).toBeVisible({ timeout: 20000 });
}

async function subscribeTheCompanyWithUsage(): Promise<void> {
	await subscribeTheCompany();
	const used = await centralPlaneAdminClient().from('company_credit_entry').insert({
		company_id: exampleCompanyID,
		member_id: member1ID,
		credit_kind: 'subscription',
		entry_kind: 'usage',
		credits: -8400
	});
	if (used.error) throw new Error(used.error.message);
}

test.afterAll(async () => {
	await forgetTheComputer();
	await unsubscribeTheCompany();
});

test('an administrator of an unsubscribed company is offered the subscription', async ({ page }) => {
	await openBilling(page);
	await expect(page.getByRole('button', { name: '구독 시작' })).toBeVisible();
	await expect(page.getByText('회사 크레딧', { exact: true })).toHaveCount(0);
	await expect(page.getByText('청구서', { exact: true })).toHaveCount(0);
});

test('an administrator of a company that no longer subscribes still sees what it was billed', async ({ page }) => {
	await page.route('**/api/stripe/invoices', (route) =>
		route.fulfill({
			json: {
				invoices: [
					{
						id: 'in_sample_refunded',
						issuedAt: '2026-10-05T00:00:00.000Z',
						description: '2 × 샘플 구독',
						amount: 40000,
						currency: 'krw',
						status: 'refunded',
						receiptURL: 'https://invoice.example.test/in_sample_refunded'
					}
				]
			}
		})
	);
	await openBilling(page);
	await expect(page.getByRole('button', { name: '구독 시작' })).toBeVisible();
	const invoices = page.locator('[data-slot="card"]').filter({ has: page.getByText('청구서', { exact: true }) });
	await expect(invoices.getByRole('row', { name: /2026.*10.*5.*₩40,000.*환불/ })).toBeVisible();
	await expect(invoices.getByRole('link', { name: '영수증' })).toHaveAttribute('href', 'https://invoice.example.test/in_sample_refunded');
});

test('an administrator of a subscribed company sees the credits and sets one limit for everybody', async ({ page }) => {
	await subscribeTheCompanyWithUsage();
	await openBilling(page);
	await expect(page.getByText('이용 중', { exact: true })).toBeVisible();
	await expect(page.getByText(keyPaysUntilNextStart)).toHaveCount(0);
	await expect(page.getByText(keyPaysNow)).toHaveCount(0);
	await expect(page.getByText('31,600 남음')).toBeVisible();
	await expect(page.getByText('8,400 / 10,000')).toBeVisible();
	await expect(page.getByRole('button', { name: '결제 관리' })).toBeVisible();
	await expect(page.getByText('청구서', { exact: true })).toBeVisible();

	const memberUsage = page.locator('[data-slot="card"]').filter({ has: page.getByText('팀원별 사용량', { exact: true }) });
	await memberUsage.getByLabel('1인 한도').fill('12000');
	await memberUsage.getByRole('button', { name: '저장', exact: true }).click();
	await expect(page.getByText('8,400 / 12,000')).toBeVisible();

	await page.reload();
	await expect(page.getByLabel('1인 한도')).toHaveValue('12000');
});

test('a member sees their own usage as a share and nothing of the billing', async ({ page }) => {
	await signInToTheCentralPlane(page, `${settingsPath}/usage`, member3Email);
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
	await openBilling(page);
	await expect(page.getByText('결제 확인 필요', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: '구독 시작' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: '결제 관리' })).toBeVisible();
});

test('a subscribed company whose computer still runs on its key is told the subscription is active and when credits take over', async ({ page }) => {
	await subscribeTheCompany();
	await companyKeyPays(false);
	await computerStartedWithItsKey();
	await openBilling(page);
	await expect(page.getByText('이용 중', { exact: true })).toBeVisible();
	await expect(page.getByText('결제 확인 필요', { exact: true })).toHaveCount(0);
	await expect(page.getByText(keyPaysUntilNextStart)).toBeVisible();
	await expect(page.getByText(keyPaysNow)).toHaveCount(0);
	await expect(page.getByText('회사 크레딧', { exact: true })).toBeVisible();
	await expect(page.getByText('자동 충전', { exact: true })).toBeVisible();
	await expect(page.getByText('팀원별 사용량', { exact: true })).toBeVisible();
});

test('a subscribed company an operator set to pay with its key is told the key pays now', async ({ page }) => {
	await subscribeTheCompany();
	await companyKeyPays(true);
	await computerStartedWithItsKey();
	await openBilling(page);
	await expect(page.getByText('이용 중', { exact: true })).toBeVisible();
	await expect(page.getByText(keyPaysNow)).toBeVisible();
	await expect(page.getByText(keyPaysUntilNextStart)).toHaveCount(0);
});

test('a subscribed company that spent the period\'s credits is told the subscription is active and that nothing is left', async ({ page }) => {
	await forgetTheComputer();
	await subscribeTheCompany();
	const spent = await centralPlaneAdminClient()
		.from('company')
		.update({ subscription_credits: 0, permanent_credits: 0 })
		.eq('id', exampleCompanyID);
	if (spent.error) throw new Error(spent.error.message);
	await openBilling(page);
	await expect(page.getByText('이용 중', { exact: true })).toBeVisible();
	await expect(page.getByText('결제 확인 필요', { exact: true })).toHaveCount(0);
	await expect(page.getByText('0 남음', { exact: true })).toBeVisible();
	await expect(page.getByText('자동 충전', { exact: true })).toBeVisible();
});

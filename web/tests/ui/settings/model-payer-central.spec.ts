import { expect, test } from '@playwright/test';
import { centralPlaneAdminClient, exampleCompanyID, exampleCompanySlug, member1Email } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { subscribeTheCompany, unsubscribeTheCompany } from './company-subscription';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const setupPath = `/${exampleCompanySlug}/settings/setup`;
const computerKeyHash = `model-payer-ui-${exampleCompanyID}`;

async function usesOwnModelKey(): Promise<boolean> {
	const { data, error } = await centralPlaneAdminClient().from('company').select('uses_own_model_key').eq('id', exampleCompanyID).single();
	if (error) throw new Error(error.message);
	return data.uses_own_model_key;
}

test.beforeAll(async () => {
	const { error } = await centralPlaneAdminClient()
		.from('agent')
		.upsert({ company_id: exampleCompanyID, name: 'company-computer', api_key_hash: computerKeyHash, revoked_at: null }, { onConflict: 'company_id,name' });
	if (error) throw new Error(error.message);
});

test.afterAll(async () => {
	const client = centralPlaneAdminClient();
	await client.from('agent').delete().eq('api_key_hash', computerKeyHash);
	await client.from('company').update({ uses_own_model_key: true }).eq('id', exampleCompanyID);
	await unsubscribeTheCompany();
});

test('an administrator sees who pays for AI and turns the company key off and on', async ({ page }) => {
	await signInToTheCentralPlane(page, setupPath, member1Email);
	const ownKey = page.getByRole('switch', { name: '우리 OpenRouter 키 사용' });
	await expect(ownKey).toBeChecked({ timeout: 20000 });
	await expect(page.getByRole('status').filter({ hasText: '지금은 AI 사용료를 낼 곳이 없습니다.' })).toBeVisible();

	await ownKey.click();
	await expect(ownKey).not.toBeChecked();
	await expect.poll(usesOwnModelKey).toBe(false);

	await page.reload();
	await expect(page.getByRole('switch', { name: '우리 OpenRouter 키 사용' })).not.toBeChecked({ timeout: 20000 });
	await page.getByRole('switch', { name: '우리 OpenRouter 키 사용' }).click();
	await expect.poll(usesOwnModelKey).toBe(true);
});

test('credits are said to pay while the company subscribes and holds no key', async ({ page }) => {
	await subscribeTheCompany();
	await signInToTheCentralPlane(page, setupPath, member1Email);
	await expect(page.getByRole('status').filter({ hasText: '지금은 회사 크레딧이 AI 사용료를 냅니다.' })).toBeVisible({ timeout: 20000 });
	await expect(page.getByRole('button', { name: '키 지우기' })).toHaveCount(0);
});

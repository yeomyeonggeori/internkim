import { expect, test } from '@playwright/test';
import { centralPlaneAdminClient, exampleCompanyID, exampleCompanySlug, member1Email } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { subscribeTheCompany, unsubscribeTheCompany } from './company-subscription';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const setupPath = `/${exampleCompanySlug}/settings/setup`;
const computerKeyHash = `model-payer-ui-${exampleCompanyID}`;
const ownKeySwitch = '우리 OpenRouter 키 사용';

async function usesOwnModelKey(): Promise<boolean> {
	const { data, error } = await centralPlaneAdminClient().from('company').select('uses_own_model_key').eq('id', exampleCompanyID).single();
	if (error) throw new Error(error.message);
	return data.uses_own_model_key;
}

async function companyUsesOwnKey(isUsed: boolean): Promise<void> {
	const { error } = await centralPlaneAdminClient().from('company').update({ uses_own_model_key: isUsed }).eq('id', exampleCompanyID);
	if (error) throw new Error(error.message);
}

async function computerStarted(report: { holdsModelKey: boolean; usesOwnModelKey: boolean } | null): Promise<void> {
	const { error } = await centralPlaneAdminClient()
		.from('agent')
		.update({
			holds_model_key: report?.holdsModelKey ?? null,
			uses_own_model_key_at_start: report?.usesOwnModelKey ?? null,
			model_key_reported_at: report ? new Date().toISOString() : null
		})
		.eq('api_key_hash', computerKeyHash);
	if (error) throw new Error(error.message);
}

test.beforeAll(async () => {
	const { error } = await centralPlaneAdminClient()
		.from('agent')
		.upsert({ company_id: exampleCompanyID, name: 'company-computer', api_key_hash: computerKeyHash, revoked_at: null }, { onConflict: 'company_id,name' });
	if (error) throw new Error(error.message);
});

test.afterAll(async () => {
	await centralPlaneAdminClient().from('agent').delete().eq('api_key_hash', computerKeyHash);
	await companyUsesOwnKey(true);
	await unsubscribeTheCompany();
});

test('an administrator turns the company key off and on before the computer has said what it runs', async ({ page }) => {
	await companyUsesOwnKey(true);
	await computerStarted(null);
	await signInToTheCentralPlane(page, setupPath, member1Email);
	const ownKey = page.getByRole('switch', { name: ownKeySwitch });
	await expect(ownKey).toBeChecked({ timeout: 20000 });
	await expect(page.getByRole('status').filter({ hasText: '아직 알리지 않았습니다.' })).toBeVisible();

	await ownKey.click();
	await expect(ownKey).not.toBeChecked();
	await expect.poll(usesOwnModelKey).toBe(false);

	await page.reload();
	await expect(page.getByRole('switch', { name: ownKeySwitch })).not.toBeChecked({ timeout: 20000 });
	await page.getByRole('switch', { name: ownKeySwitch }).click();
	await expect.poll(usesOwnModelKey).toBe(true);
});

test('a key typed into the computer is said to pay, to live there, and a change to wait for its next start', async ({ page }) => {
	await companyUsesOwnKey(true);
	await computerStarted({ holdsModelKey: true, usesOwnModelKey: true });
	await signInToTheCentralPlane(page, setupPath, member1Email);
	await expect(page.getByRole('status').filter({ hasText: '지금은 회사 OpenRouter 키가 AI 사용료를 냅니다.' })).toBeVisible({ timeout: 20000 });
	await expect(page.getByText('그 컴퓨터에만 있습니다', { exact: false })).toBeVisible();
	await expect(page.getByRole('button', { name: '키 지우기' })).toHaveCount(0);
	await expect(page.getByText('다음에 시작할 때 적용됩니다', { exact: false })).toHaveCount(0);

	await page.getByRole('switch', { name: ownKeySwitch }).click();
	await expect(page.getByText('다음에 시작할 때 적용됩니다', { exact: false })).toBeVisible();
	await expect(page.getByRole('status').filter({ hasText: '지금은 회사 OpenRouter 키가 AI 사용료를 냅니다.' })).toBeVisible();
});

test('credits are said to pay while the company subscribes and its computer holds no key', async ({ page }) => {
	await companyUsesOwnKey(true);
	await computerStarted({ holdsModelKey: false, usesOwnModelKey: true });
	await subscribeTheCompany();
	await signInToTheCentralPlane(page, setupPath, member1Email);
	await expect(page.getByRole('status').filter({ hasText: '지금은 회사 크레딧이 AI 사용료를 냅니다.' })).toBeVisible({ timeout: 20000 });
	await expect(page.getByRole('button', { name: '키 지우기' })).toHaveCount(0);
});

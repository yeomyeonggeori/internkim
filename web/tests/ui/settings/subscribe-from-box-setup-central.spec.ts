import { expect, test } from '@playwright/test';
import { centralPlaneAdminClient, exampleCompanyID, exampleCompanySlug, member1Email } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { subscribeTheCompany, unsubscribeTheCompany } from './company-subscription';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const setupPath = `/${exampleCompanySlug}/settings/setup`;
const boxPublicKey = `subscribebo${exampleCompanyID.replaceAll('-', '')}`;

test.beforeAll(async () => {
	await unsubscribeTheCompany();
	const { error } = await centralPlaneAdminClient()
		.from('box')
		.insert({ public_key: boxPublicKey, company_id: exampleCompanyID, encryption_key: 'A'.repeat(43), role: 'service' });
	if (error) throw new Error(error.message);
});

test.afterAll(async () => {
	await centralPlaneAdminClient().from('box').delete().eq('public_key', boxPublicKey);
	await unsubscribeTheCompany();
});

test('an administrator without a key subscribes from the box setup instead of pasting one', async ({ page }) => {
	await signInToTheCentralPlane(page, setupPath, member1Email);
	const subscribe = page.getByRole('button', { name: '구독하기' });
	await expect(subscribe).toBeVisible({ timeout: 20000 });
	await expect(page.getByText('키 없이 바로 시작 가능합니다.')).toBeVisible();
	await expect(page.getByLabel('OpenRouter API 키')).toBeVisible();

	await subscribeTheCompany();
	await expect(subscribe).toBeHidden({ timeout: 20000 });
	await expect(page.getByLabel('OpenRouter API 키')).toBeHidden();
});

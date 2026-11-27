import { expect, test } from '@playwright/test';
import { centralPlaneAdminClient, exampleCompanyID, exampleCompanySlug, member1Email } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { subscribeTheCompany, unsubscribeTheCompany } from './company-subscription';
import { claimABox, releaseTheClaimedBox, type ClaimedBox } from '../../support/a-claimed-box';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const setupPath = `/${exampleCompanySlug}/settings/setup`;
const ownKeySwitch = '우리 OpenRouter 키 사용';
const payerRule = '구독 중에는 크레딧이 내고, 구독이 없으면 회사 컴퓨터가 가진 키가 냅니다.';

let computer: ClaimedBox;

async function computerStarted(report: { holdsModelKey: boolean; usesOwnModelKey: boolean } | null): Promise<void> {
	const { error } = await centralPlaneAdminClient()
		.from('box')
		.update({
			holds_model_key: report?.holdsModelKey ?? null,
			uses_own_model_key_at_start: report?.usesOwnModelKey ?? null,
			model_key_reported_at: report ? new Date().toISOString() : null
		})
		.eq('public_key', computer.publicKey);
	if (error) throw new Error(error.message);
}

test.beforeAll(async () => {
	computer = await claimABox(centralPlaneAdminClient(), exampleCompanyID);
});

test.afterAll(async () => {
	await releaseTheClaimedBox(centralPlaneAdminClient(), exampleCompanyID, computer);
	await unsubscribeTheCompany();
});

test('the screen says the rule and offers no switch before the computer has said what it runs', async ({ page }) => {
	await unsubscribeTheCompany();
	await computerStarted(null);
	await signInToTheCentralPlane(page, setupPath, member1Email);
	await expect(page.getByRole('status').filter({ hasText: '아직 알리지 않았습니다.' })).toBeVisible({ timeout: 20000 });
	await expect(page.getByText(payerRule)).toBeVisible();
	await expect(page.getByRole('switch', { name: ownKeySwitch })).toHaveCount(0);
});

test('a key typed into the computer is said to pay and to live there, and subscribing waits for its next start', async ({ page }) => {
	await unsubscribeTheCompany();
	await computerStarted({ holdsModelKey: true, usesOwnModelKey: true });
	await signInToTheCentralPlane(page, setupPath, member1Email);
	await expect(page.getByRole('status').filter({ hasText: '지금은 회사 OpenRouter 키가 AI 사용료를 냅니다.' })).toBeVisible({ timeout: 20000 });
	await expect(page.getByText('그 컴퓨터에만 있습니다', { exact: false })).toBeVisible();
	await expect(page.getByRole('button', { name: '키 지우기' })).toHaveCount(0);
	await expect(page.getByText('다음에 시작할 때 적용됩니다', { exact: false })).toHaveCount(0);

	await subscribeTheCompany();
	await page.reload();
	await expect(page.getByText('다음에 시작할 때 적용됩니다', { exact: false })).toBeVisible({ timeout: 20000 });
	await expect(page.getByRole('status').filter({ hasText: '지금은 회사 OpenRouter 키가 AI 사용료를 냅니다.' })).toBeVisible();
});

test('credits are said to pay while the company subscribes and its computer holds no key', async ({ page }) => {
	await unsubscribeTheCompany();
	await computerStarted({ holdsModelKey: false, usesOwnModelKey: true });
	await subscribeTheCompany();
	await signInToTheCentralPlane(page, setupPath, member1Email);
	await expect(page.getByRole('status').filter({ hasText: '구독 사용 중' })).toBeVisible({ timeout: 20000 });
	await expect(page.getByRole('button', { name: '키 지우기' })).toHaveCount(0);
});

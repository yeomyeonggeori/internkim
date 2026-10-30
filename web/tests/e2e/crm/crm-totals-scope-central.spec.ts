import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { exampleCompanySlug, member1Email, member3Email } from '../support/central-test-utils';

test.use({ locale: 'ko-KR' });

async function openTheCRMAs(page: Page, email: string): Promise<void> {
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/crm`, email);
	await page.locator('[data-crm-ready="true"]').waitFor({ state: 'visible', timeout: 20000 });
}

test('tells a member who sees only some deals that the totals count only those', async ({ page }) => {
	await openTheCRMAs(page, member3Email);

	await expect(page.locator('[data-crm-totals-scope]')).toHaveText('합계는 볼 수 있는 거래만 셉니다.');
	await expect(page.getByText('금액 없음')).toHaveCount(0);
});

test('says nothing about scope to an administrator, who sees every deal', async ({ page }) => {
	await openTheCRMAs(page, member1Email);

	await expect(page.getByRole('tab', { name: '정의' })).toBeVisible();
	await expect(page.locator('[data-crm-totals-scope]')).toHaveCount(0);
});

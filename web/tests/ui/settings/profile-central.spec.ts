import { expect, test } from '@playwright/test';
import { exampleCompanySlug, member3Email, member3Name } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';

test.use({ locale: 'ko-KR' });

const profilePath = `/${exampleCompanySlug}/settings/profile`;
const changedPhoneNumber = '010-5550-0199';

test('a member reads who they are, and saves or takes back a phone number they changed', async ({ page }) => {
	await signInToTheCentralPlane(page, profilePath, member3Email);
	await expect(page.getByRole('heading', { level: 1, name: '내 프로필' })).toBeVisible({ timeout: 20000 });

	const name = page.getByRole('textbox', { name: '이름', exact: true });
	const phoneNumber = page.getByRole('textbox', { name: '전화번호', exact: true });
	const save = page.getByRole('button', { name: '저장', exact: true });
	const cancel = page.getByRole('button', { name: '취소', exact: true });

	await expect(name).toHaveValue(member3Name, { timeout: 20000 });
	await expect(name).toBeDisabled();
	await expect(page.getByRole('main').getByText(member3Email, { exact: true })).toBeVisible();
	await expect(save).toBeDisabled();
	await expect(cancel).toBeDisabled();
	const heldPhoneNumber = await phoneNumber.inputValue();

	await phoneNumber.fill(changedPhoneNumber);
	await expect(save).toBeEnabled();
	await cancel.click();
	await expect(phoneNumber).toHaveValue(heldPhoneNumber);
	await expect(save).toBeDisabled();

	await phoneNumber.fill(changedPhoneNumber);
	await save.click();
	await expect(page.getByText('저장했습니다.', { exact: true })).toBeVisible({ timeout: 20000 });
	await expect(save).toBeDisabled();
	await page.reload();
	await expect(phoneNumber).toHaveValue(changedPhoneNumber, { timeout: 20000 });

	await phoneNumber.fill(heldPhoneNumber);
	const restored = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith('/tools/person_update/invoke'));
	await save.click();
	expect((await restored).status()).toBe(200);
});

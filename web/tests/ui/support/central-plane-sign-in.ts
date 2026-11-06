import type { Page } from '@playwright/test';
import { member1Email, seedPassword } from './central-test-utils';
import { waitForHydration } from './hydration';

export async function signInToTheCentralPlane(
	page: Page,
	path: string,
	email: string = member1Email
): Promise<boolean> {
	await page.goto(path);
	await waitForTheSessionCheck(page);
	const emailField = page.getByRole('textbox', { name: '이메일' });
	if (!(await emailField.isVisible())) return false;
	await emailField.fill(email);
	await page.getByRole('textbox', { name: '비밀번호' }).fill(seedPassword);
	await page.getByRole('button', { name: '로그인', exact: true }).click();
	return true;
}

async function waitForTheSessionCheck(page: Page): Promise<void> {
	await waitForHydration(page);
	await page.getByTestId('web-auth-loading-status').waitFor({ state: 'detached' });
}

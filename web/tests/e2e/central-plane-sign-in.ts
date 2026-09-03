import type { Page } from '@playwright/test';
import { member1Email, seedPassword } from './central-test-utils';

export async function signInToTheCentralPlane(
	page: Page,
	path: string,
	email: string = member1Email
): Promise<void> {
	await page.goto(path);
	const emailField = page.getByRole('textbox', { name: '이메일' });
	const needsSignIn = await emailField
		.waitFor({ state: 'visible', timeout: 8000 })
		.then(() => true)
		.catch(() => false);
	if (!needsSignIn) return;
	await emailField.fill(email);
	await page.getByRole('textbox', { name: '비밀번호' }).fill(seedPassword);
	await page.getByRole('button', { name: '로그인', exact: true }).click();
}

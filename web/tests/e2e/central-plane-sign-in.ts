import type { Page } from '@playwright/test';

export async function signInToTheCentralPlane(page: Page, path: string): Promise<void> {
	await page.goto(path);
	const email = page.getByRole('textbox', { name: '이메일' });
	const needsSignIn = await email
		.waitFor({ state: 'visible', timeout: 8000 })
		.then(() => true)
		.catch(() => false);
	if (!needsSignIn) return;
	await email.fill('member1@example.com');
	await page.getByRole('textbox', { name: '비밀번호' }).fill('seed-password');
	await page.getByRole('button', { name: '로그인', exact: true }).click();
}

import { expect, test } from '@playwright/test';

const publicURL = process.env.INTERNKIM_PUBLIC_URL ?? '';

test('public URL reaches Access or Mattermost, not the removed chat UI', async ({ page }) => {
	expect(publicURL).not.toEqual('');
	const response = await page.goto(publicURL, { waitUntil: 'domcontentloaded' });
	expect(response?.status() ?? 0).toBeLessThan(500);

	const body = page.locator('body');
	await expect(body).not.toContainText('웹 채팅 UI는 제거되었습니다');
	await expect(body).not.toContainText('127.example.test');
	await expect(body).not.toContainText('Bad gateway');

	const text = await body.innerText();
	const title = await page.title();
	const html = await page.content();
	expect(`${title}\n${text}\n${html}`).toMatch(/Mattermost|Cloudflare|Access|Sign in|로그인|LoadingScreen|\/static\/main\./i);
});

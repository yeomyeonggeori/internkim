import { expect, test, type Page } from '@playwright/test';

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const adminEmail = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const adminPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';

test('local Mattermost renders and accepts an admin session', async ({ page }) => {
	const response = await page.goto(mattermostURL, { waitUntil: 'domcontentloaded' });
	expect(response?.status() ?? 0).toBeLessThan(500);

	const body = page.locator('body');
	await expect(body).not.toContainText('웹 채팅 UI는 제거되었습니다');
	await expect(body).not.toContainText('127.intern.kim');

	const browserChoice = page.getByRole('link', { name: /view in browser/i });
	if ((await browserChoice.count()) > 0) {
		await browserChoice.click();
		await page.waitForLoadState('domcontentloaded');
	}

	const loginInput = page.locator('input[name="loginId"], input[name="login_id"], input[type="email"]').first();
	const canLogin = (await loginInput.count()) > 0 && adminEmail && adminPassword;
	if (canLogin) {
		await logIn(page);
		await page.goto(mattermostPath(`/${teamName}/channels/town-square`), { waitUntil: 'domcontentloaded' });
	}

	await expect(page).toHaveTitle(/Mattermost|Intern Kim/i);
	if (!canLogin) return;

	const composer = page.locator('#post_textbox, textarea, [contenteditable="true"]').first();
	await expect(composer).toBeVisible();
});

test('InternKim boards plugin loads and renders its App Bar icon', async ({ page }) => {
	test.skip(!adminEmail || !adminPassword, 'admin credentials are required');

	await page.route('**/api/v4/properties/groups/access_control/**', (route) =>
		route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }),
	);

	const pluginLoads: string[] = [];
	page.on('console', (message) => {
		if (message.text().startsWith('Loaded plugin com.internkim.ephemeral')) {
			pluginLoads.push(message.text());
		}
	});

	await page.goto(mattermostURL, { waitUntil: 'networkidle' });
	const browserChoice = page.getByRole('link', { name: /view in browser/i });
	if ((await browserChoice.count()) > 0) {
		await browserChoice.click();
		await page.waitForLoadState('networkidle');
	}
	const loginInput = page.locator('input[name="loginId"], input[name="login_id"], input[type="email"]').first();
	if ((await loginInput.count()) > 0) {
		await logIn(page);
	}
	await page.goto(mattermostPath(`/${teamName}/channels/town-square`), { waitUntil: 'domcontentloaded' });
	await expect(page.locator('#post_textbox, textarea, [contenteditable="true"]').first()).toBeVisible();

	const pluginAppBarIcon = page.locator('#app-bar-icon-com\\.internkim\\.ephemeral [role="button"]').first();
	await expect(pluginAppBarIcon).toBeVisible();
	expect(pluginLoads.length).toBeGreaterThan(0);
});

async function logIn(page: Page): Promise<void> {
	await page.locator('input[name="loginId"], input[name="login_id"], input[type="email"]').first().fill(adminEmail);
	await page.locator('input[name="password"], input[type="password"]').first().fill(adminPassword);
	const loginResponse = page.waitForResponse(
		(response) => response.url().includes('/api/v4/users/login') && response.status() === 200,
	);
	await page.getByRole('button', { name: /sign in|log in|로그인/i }).click();
	await loginResponse;
	await page.waitForURL('**/channels/**', { timeout: 15000 });
}

function mattermostPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

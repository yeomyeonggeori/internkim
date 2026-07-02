import { expect, test, type Page } from '@playwright/test';

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const requesterLogin = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const requesterPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';
const channelName = process.env.INTERNKIM_CRUD_CHANNEL ?? '';
const prompt = process.env.INTERNKIM_APPROVE_PROMPT ?? '';
const buttonLabel = process.env.INTERNKIM_APPROVE_BUTTON ?? '확인';
const screenshotPath = process.env.INTERNKIM_CRUD_SCREENSHOT ?? '';
const buttonTimeoutMs = Number(process.env.INTERNKIM_APPROVE_TIMEOUT_MS ?? '150000');

test('press the ephemeral ask button and resume the task', async ({ page }) => {
	test.setTimeout(buttonTimeoutMs + 60000);
	await signIn(page);
	await openChannel(page);
	await postMessage(page, prompt);
	const askButton = page.getByRole('button', { name: new RegExp(`^\\s*${escapeRegExp(buttonLabel)}\\s*$`) }).first();
	await expect(askButton, `ephemeral "${buttonLabel}" button never rendered`).toBeVisible({ timeout: buttonTimeoutMs });
	await askButton.click();
	await page.waitForTimeout(3000);
	if (screenshotPath) await page.screenshot({ path: screenshotPath, fullPage: true });
});

async function postMessage(page: Page, text: string): Promise<void> {
	const box = page.locator('#post_textbox, [data-testid="post_textbox"], textarea[placeholder]').first();
	await box.waitFor({ state: 'visible', timeout: 20000 });
	await box.click();
	await box.fill(text);
	await box.press('Enter');
}

async function openChannel(page: Page): Promise<void> {
	for (let attempt = 1; attempt <= 3; attempt++) {
		await page.goto(channelPath(`/${teamName}/channels/${channelName}`), { waitUntil: 'domcontentloaded' });
		await dismissLandingPage(page);
		await page.waitForTimeout(2000);
		if ((await page.getByText(/team not found|팀을 찾을 수 없/i).count()) === 0) return;
		await signIn(page);
	}
	throw new Error(`Team Not Found for ${teamName}/${channelName}`);
}

async function signIn(page: Page): Promise<void> {
	await page.goto(mattermostURL, { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.goto(channelPath('/login'), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	const loginInput = page
		.getByRole('textbox', { name: /email or username|이메일|사용자 이름|username/i })
		.or(page.locator('input[id*="loginId" i]'))
		.first();
	await loginInput.waitFor({ state: 'visible', timeout: 20000 }).catch(() => {});
	if ((await loginInput.count()) === 0) return;
	await loginInput.fill(requesterLogin);
	await page.locator('input[type="password"]').first().fill(requesterPassword);
	await page.getByRole('button', { name: /^\s*(log in|sign in|로그인)\s*$/i }).first().click();
	await page.waitForURL((url: URL) => !url.pathname.includes('/login'), { timeout: 30000 }).catch(() => {});
}

async function dismissLandingPage(page: Page): Promise<void> {
	const viewInBrowser = page.getByRole('link', { name: /view in browser/i }).first();
	if ((await viewInBrowser.count()) > 0 && (await viewInBrowser.isVisible().catch(() => false))) {
		await viewInBrowser.click().catch(() => {});
		await page.waitForLoadState('domcontentloaded').catch(() => {});
	}
}

function channelPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

function escapeRegExp(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

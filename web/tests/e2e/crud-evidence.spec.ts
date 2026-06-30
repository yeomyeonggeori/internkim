import { expect, test } from '@playwright/test';

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const adminEmail = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const adminPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';
const channelName = process.env.INTERNKIM_CRUD_CHANNEL ?? '';
const screenshotPath = process.env.INTERNKIM_CRUD_SCREENSHOT ?? '';
const minimumPostCount = Number(process.env.INTERNKIM_CRUD_MIN_POSTS ?? '2');

test('capture 김인턴 conversation evidence', async ({ page }) => {
	test.setTimeout(90000);
	await signInAsAdmin(page);
	await page.goto(mattermostPath(`/${teamName}/channels/${channelName}`), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await dismissTutorial(page);
	await waitForRenderedPosts(page, minimumPostCount, 30000);
	await openLatestThread(page);
	await page.screenshot({ path: screenshotPath, fullPage: true });
});

async function signInAsAdmin(page): Promise<void> {
	await page.goto(mattermostURL, { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.goto(mattermostPath('/login'), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	const loginInput = page
		.getByRole('textbox', { name: /email or username|이메일|사용자 이름/i })
		.or(page.locator('input[id*="loginId" i]'))
		.first();
	if ((await loginInput.count()) === 0) return;
	await loginInput.fill(adminEmail);
	await page.locator('input[type="password"]').first().fill(adminPassword);
	await page.getByRole('button', { name: /^\s*(log in|sign in|로그인)\s*$/i }).first().click();
	await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 20000 }).catch(() => {});
}

async function dismissLandingPage(page): Promise<void> {
	const viewInBrowser = page.getByRole('link', { name: /view in browser/i }).first();
	if ((await viewInBrowser.count()) > 0 && (await viewInBrowser.isVisible().catch(() => false))) {
		await viewInBrowser.click().catch(() => {});
		await page.waitForLoadState('domcontentloaded').catch(() => {});
	}
}

async function dismissTutorial(page): Promise<void> {
	const skip = page.getByText(/no thanks|figure it out myself|건너뛰기|나중에/i).first();
	if ((await skip.count()) > 0 && (await skip.isVisible().catch(() => false))) {
		await skip.click().catch(() => {});
	}
}

async function openLatestThread(page): Promise<void> {
	const replyLink = page.getByText(/\d+\s*(repl(y|ies)|개의 답글|답글)/i).last();
	if ((await replyLink.count()) > 0 && (await replyLink.isVisible().catch(() => false))) {
		await replyLink.click().catch(() => {});
		await page.waitForTimeout(2500);
	}
}

async function waitForRenderedPosts(page, minimum: number, timeoutMs: number): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	const postLocator = page.locator('[data-testid="postView"], .post-message__text, .post');
	while (Date.now() < deadline) {
		if ((await postLocator.count()) >= minimum) {
			await page.waitForTimeout(1500);
			return;
		}
		await page.waitForTimeout(1500);
	}
}

function mattermostPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

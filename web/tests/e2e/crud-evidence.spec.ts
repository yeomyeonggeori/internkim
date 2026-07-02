import { expect, test } from '@playwright/test';

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const adminEmail = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const adminPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';
const channelName = process.env.INTERNKIM_CRUD_CHANNEL ?? '';
const channelPath = process.env.INTERNKIM_CRUD_CHANNEL_PATH ?? `/${teamName}/channels/${channelName}`;
const screenshotPath = process.env.INTERNKIM_CRUD_SCREENSHOT ?? '';
const minimumPostCount = Number(process.env.INTERNKIM_CRUD_MIN_POSTS ?? '2');

test('capture 김인턴 conversation evidence', async ({ page }) => {
	test.setTimeout(120000);
	expect(adminEmail).not.toEqual('');
	expect(adminPassword).not.toEqual('');
	expect(screenshotPath).not.toEqual('');
	await signInAsAdmin(page);
	await openChannelOrRetry(page);
	await dismissTutorial(page);
	await waitForRenderedPosts(page, minimumPostCount, 30000);
	await openLatestThread(page);
	await page.screenshot({ path: screenshotPath, fullPage: true });
});

// The requester login must actually land inside the team before we screenshot; a
// silently failed login leaves an anonymous session that renders "Team Not Found",
// which used to be saved as bogus evidence. Retry the whole login+navigation, and
// fail loudly if the team never loads so no broken screenshot is mistaken for proof.
async function openChannelOrRetry(page): Promise<void> {
	for (let attempt = 1; attempt <= 3; attempt++) {
		await page.goto(mattermostPath(channelPath), { waitUntil: 'domcontentloaded' });
		await dismissLandingPage(page);
		await page.waitForTimeout(2000);
		if ((await teamNotFoundCount(page)) === 0) return;
		await signInAsAdmin(page);
	}
	if ((await teamNotFoundCount(page)) > 0) {
		throw new Error(`Team Not Found for ${channelPath}: requester login did not land inside the team`);
	}
}

async function teamNotFoundCount(page): Promise<number> {
	return page.getByText(/team not found|팀을 찾을 수 없/i).count();
}

async function signInAsAdmin(page): Promise<void> {
	await page.goto(mattermostURL, { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.goto(mattermostPath('/login'), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	const loginInput = page
		.getByRole('textbox', { name: /email or username|이메일|사용자 이름|username/i })
		.or(page.locator('input[id*="loginId" i]'))
		.first();
	await loginInput.waitFor({ state: 'visible', timeout: 20000 }).catch(() => {});
	if ((await loginInput.count()) === 0) throw new Error('Mattermost login form not found');
	await loginInput.fill(adminEmail);
	await page.locator('input[type="password"]').first().fill(adminPassword);
	await page.getByRole('button', { name: /^\s*(log in|sign in|로그인)\s*$/i }).first().click();
	await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 30000 });
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
	const renderedPostCount = await postLocator.count();
	throw new Error(`Expected at least ${minimum} rendered posts, found ${renderedPostCount}`);
}

function mattermostPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

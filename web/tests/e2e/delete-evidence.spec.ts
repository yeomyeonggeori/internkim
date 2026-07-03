import { test, type Page } from '@playwright/test';

type MattermostPost = {
	message?: string;
	props?: {
		attachments?: unknown;
	};
};

type MattermostWindow = Window & {
	store?: {
		getState?: () => {
			entities?: {
				posts?: {
					posts?: Record<string, MattermostPost>;
				};
			};
		};
	};
};

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const requesterEmail = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const requesterPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';
const channelName = process.env.INTERNKIM_CRUD_CHANNEL ?? '';
const deletePrompt = process.env.INTERNKIM_DELETE_PROMPT ?? '';
const beforeScreenshot = process.env.INTERNKIM_DELETE_BEFORE_SHOT ?? '';
const afterScreenshot = process.env.INTERNKIM_DELETE_AFTER_SHOT ?? '';

test('capture delete approval before and after', async ({ page }) => {
	test.setTimeout(360000);
	await signIn(page);
	await page.goto(mattermostPath(`/${teamName}/channels/${channelName}`), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await dismissTutorial(page);

	await sendChannelMessage(page, deletePrompt);
	await openThreadForLatestPost(page);

	let mentionSeenAt = 0;
	let attachSeenAt = 0;
	for (let elapsed = 0; elapsed < 200 && (mentionSeenAt === 0 || attachSeenAt === 0); elapsed += 3) {
		const probe = await page.evaluate(() => {
			const store = (window as MattermostWindow).store;
			if (!store) return { mention: false, attach: false };
			const posts = Object.values(store.getState?.()?.entities?.posts?.posts ?? {});
			return {
				mention: posts.some((post) => (post.message || '').includes('진행할까요') || (post.message || '').includes('task.delete')),
				attach: posts.some((post) => Boolean(post.props?.attachments)),
			};
		}).catch(() => ({ mention: false, attach: false }));
		if (probe.mention && mentionSeenAt === 0) mentionSeenAt = elapsed;
		if (probe.attach && attachSeenAt === 0) attachSeenAt = elapsed;
		await page.waitForTimeout(3000);
	}
	console.log(`MENTION_SEEN_AT=${mentionSeenAt}s ATTACH_SEEN_AT=${attachSeenAt}s`);

	const confirmButton = page.getByRole('button', { name: /^\s*확인\s*$/ }).last();
	const rendered = (await confirmButton.count()) > 0 && (await confirmButton.isVisible().catch(() => false));
	console.log(`DELETE_BUTTON_RENDERED=${rendered}`);
	await page.waitForTimeout(1200);
	await page.screenshot({ path: beforeScreenshot, fullPage: true });

	if (rendered) {
		await confirmButton.click();
		await waitForCompletionMessage(page, 90000);
		await page.waitForTimeout(1500);
		await page.screenshot({ path: afterScreenshot, fullPage: true });
	}
});

async function openThreadForLatestPost(page: Page): Promise<void> {
	const lastPost = page.locator('[data-testid="postView"]').last();
	await lastPost.waitFor({ state: 'visible', timeout: 30000 });
	await lastPost.hover();
	const replyAction = lastPost
		.getByRole('button', { name: /^reply$/i })
		.or(lastPost.locator('button[aria-label*="reply" i]'))
		.first();
	await replyAction.click();
	await page.waitForTimeout(2000);
}

async function sendChannelMessage(page: Page, message: string): Promise<void> {
	const messageBox = page
		.locator('#post_textbox')
		.or(page.getByRole('textbox', { name: /write to|메시지/i }))
		.first();
	await messageBox.waitFor({ state: 'visible', timeout: 30000 });
	await messageBox.click();
	await messageBox.fill(message);
	await messageBox.press('Enter');
}

async function waitForCompletionMessage(page: Page, timeoutMs: number): Promise<void> {
	for (let elapsed = 0; elapsed < timeoutMs; elapsed += 3000) {
		const done = await page.evaluate(() => {
			const store = (window as MattermostWindow).store;
			if (!store) return false;
			const posts = Object.values(store.getState?.()?.entities?.posts?.posts ?? {});
			return posts.some((post) => /완료|삭제했|삭제됐|removed the|deleted the/.test(post.message || ''));
		}).catch(() => false);
		if (done) return;
		await page.waitForTimeout(3000);
	}
}

async function signIn(page: Page): Promise<void> {
	await page.goto(mattermostURL, { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.goto(mattermostPath('/login'), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	const loginInput = page
		.getByRole('textbox', { name: /email or username|이메일|사용자 이름/i })
		.or(page.locator('input[id*="loginId" i]'))
		.first();
	if ((await loginInput.count()) === 0) return;
	await loginInput.fill(requesterEmail);
	await page.locator('input[type="password"]').first().fill(requesterPassword);
	await page.getByRole('button', { name: /^\s*(log in|sign in|로그인)\s*$/i }).first().click();
	await page.waitForURL((url: URL) => !url.pathname.includes('/login'), { timeout: 20000 }).catch(() => {});
}

async function dismissLandingPage(page: Page): Promise<void> {
	const viewInBrowser = page.getByRole('link', { name: /view in browser/i }).first();
	if ((await viewInBrowser.count()) > 0 && (await viewInBrowser.isVisible().catch(() => false))) {
		await viewInBrowser.click().catch(() => {});
		await page.waitForLoadState('domcontentloaded').catch(() => {});
	}
}

async function dismissTutorial(page: Page): Promise<void> {
	const skip = page.getByText(/no thanks|figure it out myself|건너뛰기|나중에/i).first();
	if ((await skip.count()) > 0 && (await skip.isVisible().catch(() => false))) {
		await skip.click().catch(() => {});
	}
}

function mattermostPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

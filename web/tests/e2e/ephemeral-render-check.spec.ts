import { test, type APIRequestContext, type Page } from '@playwright/test';

type MattermostPost = {
	message?: string;
	props?: {
		attachments?: unknown;
	};
};

type MattermostProfile = {
	username?: string;
};

type MattermostWindow = Window & {
	store?: {
		getState?: () => {
			entities?: {
				posts?: {
					posts?: Record<string, MattermostPost>;
				};
				users?: {
					currentUserId?: string;
					profiles?: Record<string, MattermostProfile>;
				};
			};
		};
	};
};

type EphemeralPost = {
	channel_id: string;
	message: string;
	user_id: string;
	props?: {
		attachments: unknown;
	};
};

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const requesterEmail = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const requesterPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';
const channelName = process.env.INTERNKIM_CRUD_CHANNEL ?? '';
const botToken = process.env.INTERNKIM_BOT_TOKEN ?? '';
const targetUsername = process.env.INTERNKIM_TARGET_USERNAME ?? '';
const screenshotPath = process.env.INTERNKIM_RENDER_SHOT ?? '';

test('ephemeral interactive button renders live', async ({ page, request }) => {
	test.setTimeout(120000);
	await page.addInitScript(() => {
		Object.defineProperty(document, 'visibilityState', { get: () => 'visible', configurable: true });
		Object.defineProperty(document, 'hidden', { get: () => false, configurable: true });
		document.hasFocus = () => true;
	});
	let ephemeralFrameReceived = false;
	const eventTypeCounts: Record<string, number> = {};
	page.on('websocket', (webSocket) => {
		webSocket.on('framereceived', (frame) => {
			const payload = typeof frame.payload === 'string' ? frame.payload : '';
			const match = payload.match(/"event":"([a-z_]+)"/);
			if (match) eventTypeCounts[match[1]] = (eventTypeCounts[match[1]] || 0) + 1;
			if (payload.includes('ephemeral_message') || payload.includes('MANUAL_')) ephemeralFrameReceived = true;
		});
	});
	await signIn(page);
	await page.goto(mattermostPath(`/${teamName}/channels/${channelName}`), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.bringToFront();
	await page.locator('#post_textbox').first().click().catch(() => {});
	await page.waitForTimeout(4000);

	const teamID = await apiJson(request, `/api/v4/teams/name/${teamName}`, 'id');
	const channelID = await apiJson(request, `/api/v4/teams/${teamID}/channels/name/${channelName}`, 'id');
	const userID = await apiJson(request, `/api/v4/users/username/${targetUsername}`, 'id');
	const botUserID = await apiJson(request, '/api/v4/users/me', 'id');
	const webappUserID = await page.evaluate(() => {
		const store = (window as MattermostWindow).store;
		const state = store?.getState?.();
		const currentUserID = state?.entities?.users?.currentUserId ?? 'NONE';
		const username = state?.entities?.users?.profiles?.[currentUserID]?.username ?? 'NONE';
		return `${currentUserID}:${username}`;
	}).catch((error) => `EVAL_ERR:${error}`);
	console.log(`TARGET_USERID=${userID} WEBAPP_USER=${webappUserID}`);

	const plainResult = await postEphemeral(request, botToken, userID, botUserID, channelID, 'MANUAL_PLAIN_EPHEMERAL', undefined);
	const buttonsAttachment = [{ text: '주간보고서 업무를 삭제할까요?', actions: [
		{ id: 'c', name: '확인', type: 'button', style: 'primary', integration: { url: 'http://127.0.0.1:18080/_internkim/mattermost/actions', context: { action: 'ask.confirm', token: 'diagnostic' } } },
		{ id: 'x', name: '취소', type: 'button', style: 'danger', integration: { url: 'http://127.0.0.1:18080/_internkim/mattermost/actions', context: { action: 'ask.cancel', token: 'diagnostic' } } },
	] }];
	const buttonResult = await postEphemeral(request, botToken, userID, botUserID, channelID, 'MANUAL_BUTTON_EPHEMERAL', buttonsAttachment);
	console.log(`POST_STATUS plain=${plainResult} buttons=${buttonResult}`);

	let plainDelivered = false;
	let attachDelivered = false;
	for (let elapsed = 0; elapsed < 20 && (!plainDelivered || !attachDelivered); elapsed += 2) {
		const probe = await page.evaluate(() => {
			const store = (window as MattermostWindow).store;
			if (!store) return { plain: false, attach: false };
			const posts = Object.values(store.getState?.()?.entities?.posts?.posts ?? {});
			return {
				plain: posts.some((post) => (post.message || '').includes('MANUAL_PLAIN_EPHEMERAL') || (post.message || '').includes('MANUAL_BUTTON_EPHEMERAL')),
				attach: posts.some((post) => Boolean(post.props?.attachments)),
			};
		}).catch(() => ({ plain: false, attach: false }));
		plainDelivered = plainDelivered || probe.plain;
		attachDelivered = attachDelivered || probe.attach;
		await page.waitForTimeout(2000);
	}
	const confirmButton = page.getByRole('button', { name: /^\s*확인\s*$/ }).last();
	const rendered = (await confirmButton.count()) > 0 && (await confirmButton.isVisible().catch(() => false));
	await page.screenshot({ path: screenshotPath, fullPage: true });
	console.log(`WEBAPP_WS_FRAME_RECEIVED=${ephemeralFrameReceived} PLAIN_DELIVERED=${plainDelivered} ATTACH_DELIVERED=${attachDelivered} BUTTON_RENDERED=${rendered}`);
});

async function postEphemeral(
	request: APIRequestContext,
	token: string,
	userID: string,
	botUserID: string,
	channelID: string,
	message: string,
	attachments: unknown
): Promise<number> {
	const post: EphemeralPost = { channel_id: channelID, message, user_id: botUserID };
	if (attachments) post.props = { attachments };
	const response = await request.post(mattermostPath('/api/v4/posts/ephemeral'), {
		headers: { Authorization: `Bearer ${token}` },
		data: { user_id: userID, post },
	});
	return response.status();
}

async function apiJson(request: APIRequestContext, path: string, field: string): Promise<string> {
	const response = await request.get(mattermostPath(path), { headers: { Authorization: `Bearer ${botToken}` } });
	const body: unknown = await response.json();
	if (!isRecord(body) || typeof body[field] !== 'string') {
		throw new Error(`Mattermost API response did not include string field "${field}" for ${path}`);
	}
	return body[field];
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

function mattermostPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

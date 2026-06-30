import { test } from '@playwright/test';

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL ?? 'http://127.0.0.1:8065';
const requesterEmail = process.env.INTERNKIM_ADMIN_EMAIL ?? '';
const requesterPassword = process.env.INTERNKIM_ADMIN_PASSWORD ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME ?? 'internkim';
const channelName = process.env.INTERNKIM_CRUD_CHANNEL ?? '';
const botToken = process.env.INTERNKIM_BOT_TOKEN ?? '';
const targetUsername = process.env.INTERNKIM_TARGET_USERNAME ?? '';
const screenshotPath = process.env.INTERNKIM_MATRIX_SHOT ?? '';

const actionURL = `${mattermostURL.replace(/:8065.*/, ':18080')}/_internkim/mattermost/actions`;
const buttons = [{ text: '버튼 첨부', actions: [
	{ id: 'c', name: '확인', type: 'button', style: 'primary', integration: { url: actionURL, context: { action: 'ask.confirm', token: 'diagnostic' } } },
	{ id: 'x', name: '취소', type: 'button', style: 'danger', integration: { url: actionURL, context: { action: 'ask.cancel', token: 'diagnostic' } } },
] }];

test('matrix of four message kinds', async ({ page, request }) => {
	test.setTimeout(90000);
	page.on('websocket', (webSocket) => console.log(`WS_URL=${webSocket.url()}`));
	await signIn(page);
	await page.goto(mattermostPath(`/${teamName}/channels/${channelName}`), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.bringToFront();
	await page.locator('#post_textbox').first().click().catch(() => {});
	await page.waitForTimeout(4000);

	const teamID = await apiJson(request, `/api/v4/teams/name/${teamName}`, 'id');
	const channelID = await apiJson(request, `/api/v4/teams/${teamID}/channels/name/${channelName}`, 'id');
	const userID = await apiJson(request, `/api/v4/users/username/${targetUsername}`, 'id');

	const s1 = await postNormal(request, channelID, 'MATRIX-1 일반 메시지 (텍스트만)', undefined);
	const s2 = await postNormal(request, channelID, 'MATRIX-2 일반 메시지 + 버튼', buttons);
	const s3 = await postEphemeral(request, userID, channelID, 'MATRIX-3 ephemeral (텍스트만)', undefined);
	const s4 = await postEphemeral(request, userID, channelID, 'MATRIX-4 ephemeral + 버튼', buttons);
	console.log(`POST_STATUS normal=${s1} normalButtons=${s2} ephemeral=${s3} ephemeralButtons=${s4}`);

	await page.waitForTimeout(6000);
	await page.screenshot({ path: screenshotPath, fullPage: true });
});

async function postNormal(request, channelID: string, message: string, attachments: unknown): Promise<number> {
	const data: any = { channel_id: channelID, message };
	if (attachments) data.props = { attachments };
	const response = await request.post(mattermostPath('/api/v4/posts'), { headers: { Authorization: `Bearer ${botToken}` }, data });
	return response.status();
}

async function postEphemeral(request, userID: string, channelID: string, message: string, attachments: unknown): Promise<number> {
	const post: any = { channel_id: channelID, message };
	if (attachments) post.props = { attachments };
	const response = await request.post(mattermostPath('/api/v4/posts/ephemeral'), { headers: { Authorization: `Bearer ${botToken}` }, data: { user_id: userID, post } });
	return response.status();
}

async function apiJson(request, path: string, field: string): Promise<string> {
	const response = await request.get(mattermostPath(path), { headers: { Authorization: `Bearer ${botToken}` } });
	const body = await response.json();
	return body[field];
}

async function signIn(page): Promise<void> {
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
	await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 20000 }).catch(() => {});
}

async function dismissLandingPage(page): Promise<void> {
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

import { expect, test, type Page } from '@playwright/test';
import { waitForHydration } from '../support/hydration';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const email = 'member@example.com';
const readerExternalID = 'a'.repeat(64);
const directory = [{ id: memberID, name: '이샘플', email, messenger: { buzz: readerExternalID }, is_admin: true }];
const link = 'https://example.com/reel/one';
const channel = { id: 'channel-notice', name: '공지', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID] };
const linkMessage = { id: 'link-message', conversationID: channel.id, authorExternalID: readerExternalID, body: link, postedAt: '2026-10-27T01:00:00.000Z', reactions: [], attachments: [] };

type LinkVideo = { url: string; kind: 'file' | 'player'; width: number; height: number };
type LinkPreview = { url: string; title: string; description: string; siteName: string; imageURL: string; video: LinkVideo | null };

const preview: LinkPreview = {
	url: link,
	title: '샘플 영상',
	description: '좋아요 10개',
	siteName: 'Example',
	imageURL: 'https://cdn.example.com/cover.jpg',
	video: null
};
const videoFile: LinkVideo = { url: 'https://cdn.example.com/one.mp4', kind: 'file', width: 720, height: 1280 };
const videoPlayer: LinkVideo = { url: 'https://player.example.com/embed/one', kind: 'player', width: 0, height: 0 };
const poster = '<svg xmlns="http://www.w3.org/2000/svg" width="191" height="100"><rect width="191" height="100" fill="#466baa"/></svg>';

async function signInWithOneMessage(page: Page, preview: LinkPreview): Promise<void> {
	await page.addInitScript(({ memberID, email }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({ access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`, refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600, user: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } }));
	}, { memberID, email });
	await page.route('http://127.0.0.1:56801/**', async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname === '/auth/v1/user') return route.fulfill({ json: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } });
		if (url.pathname !== '/rest/v1/member') return route.fulfill({ json: [] });
		if (url.searchParams.has('status')) return route.fulfill({ json: directory });
		const reader = { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', messenger: { buzz: readerExternalID }, company: { slug: 'example-co', locale: 'ko' } };
		const asksForOneRow = (route.request().headers()['accept'] ?? '').includes('vnd.pgrst.object');
		await route.fulfill({ json: asksForOneRow ? reader : [reader] });
	});
	await page.route('**/api/member/me', (route) => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email, isAdmin: true } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/agent/api/**', (route) => route.fulfill({ json: {} }));
	await page.routeWebSocket('**/company/*/client', (socket) => {
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: true }));
		socket.onMessage((raw) => {
			const message: { kind: string; capability: string; requestID: string } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			const answer = (body: object) => socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status: 200, body }));
			if (message.capability === 'person.conversations.list') return answer({ conversations: [channel] });
			if (message.capability === 'person.identity') return answer({ externalID: readerExternalID, name: '이샘플' });
			if (message.capability === 'person.messages.list') return answer({ messages: [linkMessage], hasMoreBefore: false });
			if (message.capability === 'person.inbox.list') return answer({ items: [] });
			if (message.capability === 'asset.link') return answer(preview);
			answer({});
		});
	});
}
const card = (page: Page) => page.locator(`a[href="${link}"]`).filter({ hasText: preview.title });

test.use({ locale: 'ko-KR' });

test('a link card whose picture loads shows it above the title', async ({ page }) => {
	await signInWithOneMessage(page, preview);
	await page.route(preview.imageURL, (route) => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="191" height="100"/>' }));
	await page.goto(`/messenger?channel=${channel.id}`);
	await waitForHydration(page);
	await expect(card(page).locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'loaded');
	await expect(card(page)).toContainText(preview.siteName);
});

test('a link card whose picture the site refuses keeps its text and leaves no broken frame', async ({ page }) => {
	await signInWithOneMessage(page, preview);
	await page.route(preview.imageURL, (route) => route.fulfill({ status: 403, body: 'refused' }));
	await page.goto(`/messenger?channel=${channel.id}`);
	await waitForHydration(page);
	await expect(card(page)).toContainText(preview.description);
	await expect(card(page).locator('[data-loading-image]')).toHaveCount(0);
	await expect(card(page).getByText('이미지를 불러올 수 없어요')).toHaveCount(0);
});

test('a link card with a video file plays it in place and keeps its link', async ({ page }) => {
	await signInWithOneMessage(page, { ...preview, video: videoFile });
	await page.route(preview.imageURL, (route) => route.fulfill({ contentType: 'image/svg+xml', body: poster }));
	await page.route(videoFile.url, () => {});
	await page.goto(`/messenger?channel=${channel.id}`);
	await waitForHydration(page);
	const video = page.locator('[data-link-video="file"]');
	await expect(video.locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'loaded');
	const box = await video.boundingBox();
	if (!box) throw new Error('The link video has no frame');
	expect(box.width / box.height).toBeCloseTo(720 / 1280, 2);
	expect(box.height).toBeLessThanOrEqual(26 * 16 + 1);
	await page.getByRole('button', { name: `${preview.title} 영상 재생` }).click();
	await expect(video.locator('video')).toHaveAttribute('src', videoFile.url);
	await expect(card(page)).toContainText(preview.description);
});

test('a link card with a site player frames the player only once asked to play', async ({ page }) => {
	await signInWithOneMessage(page, { ...preview, video: videoPlayer });
	await page.route(preview.imageURL, (route) => route.fulfill({ contentType: 'image/svg+xml', body: poster }));
	let isPlayerRequested = false;
	await page.route(videoPlayer.url, (route) => {
		isPlayerRequested = true;
		return route.fulfill({ contentType: 'text/html', body: '<p>player</p>' });
	});
	await page.goto(`/messenger?channel=${channel.id}`);
	await waitForHydration(page);
	const frame = page.locator('[data-link-video="player"]');
	await expect(frame.locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'loaded');
	expect(isPlayerRequested).toBe(false);
	await frame.getByRole('button', { name: `${preview.title} 영상 재생` }).click();
	await expect(page.frameLocator(`iframe[title="${preview.title}"]`).getByText('player')).toBeVisible();
	await expect(card(page)).toContainText(preview.siteName);
});

test('a link card whose video file fails goes back to the picture card', async ({ page }) => {
	await signInWithOneMessage(page, { ...preview, video: videoFile });
	await page.route(preview.imageURL, (route) => route.fulfill({ contentType: 'image/svg+xml', body: poster }));
	await page.route(videoFile.url, (route) => route.fulfill({ status: 404 }));
	await page.goto(`/messenger?channel=${channel.id}`);
	await waitForHydration(page);
	await page.getByRole('button', { name: `${preview.title} 영상 재생` }).click();
	await expect(page.locator('[data-link-video]')).toHaveCount(0);
	await expect(card(page).locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'loaded');
});

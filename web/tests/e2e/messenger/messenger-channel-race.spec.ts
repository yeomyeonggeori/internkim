import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger, type MockConversation } from './messenger-device-mock';

const reader = { id: 'sample-reader', email: 'reader@example.com' };
const channels: MockConversation[] = ['channel-a', 'channel-b', 'channel-c'].map((id) => ({
	id,
	name: id,
	kind: 'group',
	myRole: 'member'
}));
const message = (id: string) => ({
	id,
	sender: { id: 'sample-sender', name: '이샘플' },
	text: id,
	sentAt: '2026-08-05T10:00:00Z'
});

function readGate() {
	let release = () => {};
	const opened = new Promise<void>((resolve) => (release = resolve));
	return { opened, release };
}

function emptyRead(channelID: string) {
	return { json: { conversationID: channelID, currentUserId: reader.id, messages: [], hasMoreBefore: false, historyCursor: '' } };
}

test('an old channel read cannot erase the cache after rapid navigation', async ({ page }) => {
	await mockDeviceMessenger(page, reader, channels, []);
	let oldReadStarted = () => {};
	const oldReadIsStarted = new Promise<void>((resolve) => (oldReadStarted = resolve));
	let releaseOldRead = () => {};
	const oldReadCanFinish = new Promise<void>((resolve) => (releaseOldRead = resolve));
	let releaseFinalRead = () => {};
	const finalReadCanFinish = new Promise<void>((resolve) => (releaseFinalRead = resolve));
	let isHoldingReads = false;
	let aReads = 0;
	await page.route('**/agent/api/dm?*', async (route) => {
		const channelID = new URL(route.request().url()).searchParams.get('channelId') ?? '';
		if (channelID === 'channel-a' && ++aReads === 2) {
			oldReadStarted();
			await oldReadCanFinish;
			await route.fulfill({ json: { conversationID: channelID, currentUserId: reader.id, messages: [], hasMoreBefore: false, historyCursor: '' } });
			return;
		}
		if (channelID === 'channel-a' && isHoldingReads) await finalReadCanFinish;
		await route.fulfill({
			json: {
				conversationID: channelID,
				currentUserId: reader.id,
				messages: channelID === 'channel-a' ? [message('a-message')] : [message(`${channelID}-message`)],
				hasMoreBefore: false,
				historyCursor: ''
			}
		});
	});
	await page.goto('/messenger?channel=channel-a');
	await expect(page.getByText('a-message')).toBeVisible();
	await page.clock.install();
	await page.clock.fastForward(5000);
	await oldReadIsStarted;
	for (const id of ['channel-b', 'channel-c', 'channel-a']) {
		await page.locator('[data-sidebar="menu-button"]').filter({ hasText: id }).click();
		await expect(page).toHaveURL(new RegExp(`channel=${id}`));
	}
	await expect(page.getByText('a-message')).toBeVisible();
	const oldResponse = page.waitForResponse((response) =>
		response.url().includes('channelId=channel-a') && response.status() === 200
	);
	isHoldingReads = true;
	releaseOldRead();
	await oldResponse;
	await page.waitForTimeout(100);
	expect(aReads).toBe(3);
	await page.locator('[data-sidebar="menu-button"]').filter({ hasText: 'channel-b' }).click();
	await page.locator('[data-sidebar="menu-button"]').filter({ hasText: 'channel-a' }).click();
	expect(aReads).toBe(4);
	await expect(page.getByText('a-message')).toBeVisible();
	await page.reload();
	await expect.poll(() => aReads).toBe(5);
	await expect(page.getByText('a-message')).toBeVisible();
	releaseFinalRead();
});

test('an older empty read cannot replace a newer successful read in the active channel', async ({ page }) => {
	await mockDeviceMessenger(page, reader, channels, []);
	let oldReadStarted = () => {};
	const oldReadIsStarted = new Promise<void>((resolve) => (oldReadStarted = resolve));
	let releaseOldRead = () => {};
	const oldReadCanFinish = new Promise<void>((resolve) => (releaseOldRead = resolve));
	const reopenedReads = readGate();
	let isHoldingReads = false;
	let readCount = 0;
	const knownMessage = {
		...message('known-message'),
		interaction: { kind: 'choice', question: '', options: [{ key: 'answer', label: '응답' }] }
	};
	await page.route('**/agent/api/dm?*', async (route) => {
		if (route.request().method() === 'POST') {
			await route.fulfill({ json: {} });
			return;
		}
		const channelID = new URL(route.request().url()).searchParams.get('channelId') ?? '';
		if (channelID !== 'channel-a') {
			await route.fulfill(emptyRead(channelID));
			return;
		}
		readCount += 1;
		if (readCount === 2) {
			oldReadStarted();
			await oldReadCanFinish;
			await route.fulfill(emptyRead('channel-a'));
			return;
		}
		if (isHoldingReads) await reopenedReads.opened;
		await route.fulfill({
			json: {
				conversationID: 'channel-a',
				currentUserId: reader.id,
				messages: readCount >= 3 ? [knownMessage, message('newer-message')] : [knownMessage],
				hasMoreBefore: false,
				historyCursor: ''
			}
		});
	});
	await page.goto('/messenger?channel=channel-a');
	await expect(page.getByText('known-message')).toBeVisible();
	await oldReadIsStarted;
	await page.getByRole('button', { name: '응답' }).click();
	await expect(page.getByText('newer-message')).toBeVisible();
	const oldResponse = page.waitForResponse((response) =>
		response.url().includes('channelId=channel-a') && response.status() === 200
	);
	isHoldingReads = true;
	releaseOldRead();
	await oldResponse;
	await page.waitForTimeout(100);
	await expect(page.getByText('newer-message')).toBeVisible();
	const reads = readCount;
	await page.reload();
	await expect.poll(() => readCount).toBeGreaterThan(reads);
	await expect(page.getByText('known-message')).toBeVisible();
	await expect(page.getByText('newer-message')).toBeVisible();
	reopenedReads.release();
});

for (const newerRead of ['pending', 'failed']) {
	test(`an older empty read cannot erase messages when the newer read is ${newerRead}`, async ({ page }) => {
		await mockDeviceMessenger(page, reader, channels, []);
		let releaseOlderRead = () => {};
		const olderReadCanFinish = new Promise<void>((resolve) => (releaseOlderRead = resolve));
		let olderReadStarted = () => {};
		const olderReadIsStarted = new Promise<void>((resolve) => (olderReadStarted = resolve));
		let releaseNewerRead = () => {};
		const newerReadCanFinish = new Promise<void>((resolve) => (releaseNewerRead = resolve));
		const reopenedReads = readGate();
		let isHoldingReads = false;
		let readCount = 0;
		const knownMessage = {
			...message('known-message'),
			interaction: { kind: 'choice', question: '', options: [{ key: 'answer', label: '응답' }] }
		};
		await page.route('**/agent/api/dm?*', async (route) => {
			if (route.request().method() === 'POST') {
				await route.fulfill({ json: {} });
				return;
			}
			const channelID = new URL(route.request().url()).searchParams.get('channelId') ?? '';
			if (channelID !== 'channel-a') {
				await route.fulfill(emptyRead(channelID));
				return;
			}
			readCount += 1;
			if (readCount === 2) {
				olderReadStarted();
				await olderReadCanFinish;
				await route.fulfill(emptyRead('channel-a'));
				return;
			}
			if (isHoldingReads) await reopenedReads.opened;
			if (readCount === 3 && newerRead === 'pending') await newerReadCanFinish;
			if (readCount === 3 && newerRead === 'failed') {
				await route.fulfill({ status: 503, body: 'unavailable' });
				return;
			}
			await route.fulfill({ json: { conversationID: 'channel-a', currentUserId: reader.id, messages: [knownMessage], hasMoreBefore: false, historyCursor: '' } });
		});
		await page.goto('/messenger?channel=channel-a');
		await expect(page.getByText('known-message')).toBeVisible();
		await page.clock.install();
		await page.clock.fastForward(5000);
		await olderReadIsStarted;
		const failedResponse = newerRead === 'failed'
			? page.waitForResponse((response) => response.url().includes('channelId=channel-a') && response.status() === 503)
			: null;
		await page.getByRole('button', { name: '응답' }).click();
		await expect.poll(() => readCount).toBe(3);
		if (failedResponse) await failedResponse;
		const olderResponse = page.waitForResponse((response) =>
			response.url().includes('channelId=channel-a') && response.status() === 200
		);
		isHoldingReads = true;
		releaseOlderRead();
		await olderResponse;
		await page.waitForTimeout(100);
		await expect(page.getByText('known-message')).toBeVisible();
		const reads = readCount;
		await page.reload();
		await expect.poll(() => readCount).toBeGreaterThan(reads);
		await expect(page.getByText('known-message')).toBeVisible();
		releaseNewerRead();
		reopenedReads.release();
	});
}

test('an actual empty channel and a failed channel still show their own states', async ({ page }) => {
	await mockDeviceMessenger(page, reader, channels, []);
	await page.route('**/agent/api/dm?*', async (route) => {
		const channelID = new URL(route.request().url()).searchParams.get('channelId');
		if (channelID === 'channel-b') {
			await route.fulfill({ status: 503, body: 'unavailable' });
			return;
		}
		await route.fulfill({
			json: {
				conversationID: channelID,
				currentUserId: reader.id,
				messages: channelID === 'channel-a' ? [message('a-message')] : [],
				hasMoreBefore: false,
				historyCursor: ''
			}
		});
	});
	await page.goto('/messenger?channel=channel-a');
	await expect(page.getByText('a-message')).toBeVisible();
	await page.locator('[data-sidebar="menu-button"]').filter({ hasText: 'channel-b' }).click();
	await expect(page.getByText('김인턴과 연결할 수 없습니다')).toBeVisible();
	await page.locator('[data-sidebar="menu-button"]').filter({ hasText: 'channel-c' }).click();
	await expect(page.getByText('아직 대화가 없어요')).toBeVisible();
	await page.locator('[data-sidebar="menu-button"]').filter({ hasText: 'channel-a' }).click();
	await expect(page.getByText('a-message')).toBeVisible();
});

test('loading older messages survives a newer refresh of the same channel', async ({ page }) => {
	await mockDeviceMessenger(page, reader, channels, []);
	const recent = Array.from({ length: 50 }, (_, index) => message(`recent-${index}`));
	let olderReadStarted = () => {};
	const olderReadIsStarted = new Promise<void>((resolve) => (olderReadStarted = resolve));
	let releaseOlderRead = () => {};
	const olderReadCanFinish = new Promise<void>((resolve) => (releaseOlderRead = resolve));
	await page.route('**/agent/api/dm?*', async (route) => {
		const before = new URL(route.request().url()).searchParams.get('before');
		if (before === 'older-cursor') {
			olderReadStarted();
			await olderReadCanFinish;
			await route.fulfill({ json: { conversationID: 'channel-a', currentUserId: reader.id, messages: [message('older-message')], hasMoreBefore: false, historyCursor: '' } });
			return;
		}
		await route.fulfill({ json: { conversationID: 'channel-a', currentUserId: reader.id, messages: recent, hasMoreBefore: true, historyCursor: 'older-cursor' } });
	});
	await page.setViewportSize({ width: 1280, height: 700 });
	await page.goto('/messenger?channel=channel-a');
	await expect(page.getByText('recent-49')).toBeVisible();
	await page.locator('.overscroll-y-none').evaluate((element) => {
		element.scrollTop = -element.scrollHeight;
		element.dispatchEvent(new Event('scroll'));
	});
	await olderReadIsStarted;
	await page.clock.install();
	await page.clock.fastForward(5000);
	await expect(page.getByText('recent-49')).toBeVisible();
	releaseOlderRead();
	await expect(page.getByText('older-message')).toBeVisible();
	await expect(page.getByText('recent-49')).toBeVisible();
});

import { expect, test, type Page } from '@playwright/test';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from './workspace-loading-fixture';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

const postedAt = '2026-10-06T02:00:00.000Z';

const conversations = [
	{ id: 'channel-1', name: '광장', kind: 'group', isPrivate: false, participantExternalIDs: ['pk-park', 'pk-choi'] },
	{ id: 'dm-1', name: '박예시', kind: 'dm', isPrivate: true, participantExternalIDs: ['pk-park'] }
];

const people = [
	{ externalID: 'pk-park', name: '박예시' },
	{ externalID: 'pk-choi', name: '최견본' }
];

const members = [
	{ id: '30000000-0000-4000-8000-000000000002', name: '박예시', email: 'park@example.com', messenger: { buzz: 'pk-park' }, is_admin: false },
	{ id: '30000000-0000-4000-8000-000000000003', name: '최견본', email: 'choi@example.com', messenger: { buzz: 'pk-choi' }, is_admin: false }
];

const messages = {
	'channel-1': [
		{ id: 'root-1', conversationID: 'channel-1', authorExternalID: 'pk-park', body: '배포 일정 공유드려요', postedAt: '2026-10-06T01:00:00.000Z', reactions: [], attachments: [] },
		{ id: 'reply-1', conversationID: 'channel-1', parentID: 'root-1', authorExternalID: 'pk-park', body: '로그인 화면 고쳤어요', postedAt, reactions: [], attachments: [] },
		{ id: 'mention-1', conversationID: 'channel-1', authorExternalID: 'pk-choi', body: '@이샘플 확인 부탁드려요', postedAt: '2026-10-06T01:30:00.000Z', reactions: [], attachments: [] }
	],
	'dm-1': [
		{ id: 'direct-1', conversationID: 'dm-1', authorExternalID: 'pk-park', body: '감사합니다', postedAt: '2026-10-05T01:00:00.000Z', reactions: [], attachments: [] }
	]
};

const inbox = [
	{ reason: 'thread_reply', isUnread: true, message: { id: 'reply-1', conversationID: 'channel-1', parentID: 'root-1', authorExternalID: 'pk-park', body: '로그인 화면 고쳤어요', postedAt } },
	{ reason: 'mention', isUnread: true, message: { id: 'mention-1', conversationID: 'channel-1', authorExternalID: 'pk-choi', body: '@이샘플 확인 부탁드려요', postedAt: '2026-10-06T01:30:00.000Z' } },
	{ reason: 'direct', isUnread: false, message: { id: 'direct-1', conversationID: 'dm-1', authorExternalID: 'pk-park', body: '감사합니다', postedAt: '2026-10-05T01:00:00.000Z' } }
];

class MessengerInboxFixture extends WorkspaceLoadingFixture {
	readonly marks: Record<string, unknown>[] = [];

	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		if (capability === 'person.conversations.list') return { status: 200, body: { conversations } };
		if (capability === 'person.people.list') return { status: 200, body: { people } };
		if (capability === 'person.inbox.list') return { status: 200, body: { items: inbox } };
		if (capability === 'person.messages.list') {
			const conversationID = String(body.conversationID);
			return { status: 200, body: { messages: conversationID === 'channel-1' ? messages['channel-1'] : messages['dm-1'], hasMoreBefore: false } };
		}
		if (capability === 'person.read_state.mark') {
			this.marks.push(body);
			return { status: 200, body: {} };
		}
		return super.host(capability, body);
	}
}

class LongInboxFixture extends MessengerInboxFixture {
	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		if (capability !== 'person.inbox.list') return super.host(capability, body);
		const olderEntries = Array.from({ length: 40 }, (_, index) => ({
			reason: 'direct',
			isUnread: true,
			message: { id: `older-${index}`, conversationID: 'dm-1', authorExternalID: 'pk-park', body: `지난 메시지 ${index}`, postedAt: new Date(Date.parse('2026-10-04T01:00:00.000Z') - index * 60_000).toISOString() }
		}));
		return { status: 200, body: { items: [...inbox, ...olderEntries] } };
	}
}

async function openMessenger(page: Page, width: number, fixture = new MessengerInboxFixture()): Promise<MessengerInboxFixture> {
	await page.setViewportSize({ width, height: 900 });
	await prepareWorkspaceLoading(page, fixture);
	await page.route('http://127.0.0.1:56801/rest/v1/member?**', async (route) => {
		if (route.request().url().includes('user_id=eq.')) return route.fallback();
		await route.fulfill({ json: members });
	});
	await page.goto('/messenger');
	return fixture;
}

function conversationHeader(page: Page) {
	return page.locator('header', { has: page.getByRole('button', { name: '대화 검색' }) }).getByText('광장', { exact: true });
}

function inboxPanel(page: Page) {
	return page.getByRole('complementary', { name: '수신함' });
}

test('the inbox opens first and gathers threads, mentions and direct messages newest first', async ({ page }) => {
	await openMessenger(page, 1280);

	const panel = inboxPanel(page);
	await expect(panel.locator('[data-inbox-entry]')).toHaveText([/박예시.*스레드 답장 · # 광장.*로그인 화면 고쳤어요/s, /최견본.*멘션 · # 광장/s, /박예시.*개인 메시지.*감사합니다/s]);
	const inboxButton = page.getByRole('button', { name: '수신함', exact: true });
	await expect(inboxButton).toHaveAttribute('data-active', 'true');
	await expect(page.getByRole('listitem').filter({ has: inboxButton })).toContainText('2');

	await panel.getByRole('tab', { name: '안 읽음' }).click();
	await expect(panel.locator('[data-inbox-entry]')).toHaveCount(2);
});

test('choosing a thread reply opens its conversation and marks only that thread read', async ({ page }) => {
	const fixture = await openMessenger(page, 1280);

	await inboxPanel(page).locator('[data-inbox-entry="reply-1"]').click();

	await expect(conversationHeader(page)).toBeVisible();
	await expect.poll(() => fixture.marks).toContainEqual({ conversationID: 'channel-1', parentID: 'root-1', readAt: postedAt });
	const inboxButton = page.getByRole('button', { name: '수신함', exact: true });
	await expect(page.getByRole('listitem').filter({ has: inboxButton })).toContainText('1');
});

test('choosing a channel from the sidebar leaves the inbox', async ({ page }) => {
	await openMessenger(page, 1280);
	await expect(inboxPanel(page)).toBeVisible();

	await page.getByRole('button', { name: '광장', exact: true }).click();

	await expect(inboxPanel(page)).toBeHidden();
	await expect(conversationHeader(page)).toBeVisible();
});

test('on a phone the inbox fills the screen until an entry is chosen', async ({ page }) => {
	await openMessenger(page, 390);
	const panel = inboxPanel(page);
	await expect(panel).toBeVisible();

	await panel.locator('[data-inbox-entry="direct-1"]').click();

	await expect(panel).toBeHidden();
	await expect(page.getByText('감사합니다').first()).toBeVisible();
});

test('on a phone the conversation behind the inbox is not marked read', async ({ page }) => {
	const fixture = await openMessenger(page, 390);
	await expect(inboxPanel(page).locator('[data-inbox-entry]')).toHaveCount(3);
	await expect.poll(() => fixture.requested.has('person.messages.list')).toBe(true);
	await page.waitForTimeout(1000);

	expect(fixture.marks).toEqual([]);

	await inboxPanel(page).locator('[data-inbox-entry="mention-1"]').click();
	await expect.poll(() => fixture.marks.map((mark) => mark.conversationID)).toContain('channel-1');
});

test('a long inbox stays inside its panel instead of lengthening the page', async ({ page }) => {
	await openMessenger(page, 1280, new LongInboxFixture());
	await expect(inboxPanel(page).locator('[data-inbox-entry]')).toHaveCount(43);

	await inboxPanel(page).locator('[data-inbox-entry="direct-1"]').click();

	const documentBox = await page.evaluate(() => ({ scrollHeight: document.documentElement.scrollHeight, clientHeight: document.documentElement.clientHeight, scrollTop: document.documentElement.scrollTop }));
	expect(documentBox.scrollHeight).toBe(documentBox.clientHeight);
	expect(documentBox.scrollTop).toBe(0);
});

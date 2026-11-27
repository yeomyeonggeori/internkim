import { expect, test, type Page } from '@playwright/test';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from '../shell/workspace-loading-fixture';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

type Conversation = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	isHidden?: boolean;
	isPrivate: boolean;
	participantExternalIDs: string[];
};

const readerID = '10000000-0000-4000-8000-000000000001';

const people = [
	{ externalID: 'pk-park', name: '박예시' },
	{ externalID: 'pk-choi', name: '최견본' }
];

const members = [
	{ id: '30000000-0000-4000-8000-000000000002', name: '박예시', email: 'park@example.com', messenger: { buzz: 'pk-park' }, is_admin: false },
	{ id: '30000000-0000-4000-8000-000000000003', name: '최견본', email: 'choi@example.com', messenger: { buzz: 'pk-choi' }, is_admin: false }
];

const square: Conversation = { id: 'channel-1', name: '광장', kind: 'group', isPrivate: false, participantExternalIDs: ['pk-park', 'pk-choi'] };
const withPark: Conversation = { id: 'dm-park', name: '박예시', kind: 'dm', isPrivate: true, participantExternalIDs: ['pk-park'] };
const withChoi: Conversation = { id: 'dm-choi', name: '최견본', kind: 'dm', isPrivate: true, participantExternalIDs: ['pk-choi'] };

const messages: Record<string, object[]> = {
	'channel-1': [
		{ id: 'root-1', conversationID: 'channel-1', authorExternalID: 'pk-park', body: '배포 일정 공유드려요', postedAt: '2026-10-06T01:00:00.000Z', reactions: [], attachments: [] },
		{ id: 'reply-1', conversationID: 'channel-1', parentID: 'root-1', authorExternalID: 'pk-park', body: '로그인 화면 고쳤어요', postedAt: '2026-10-06T02:00:00.000Z', reactions: [], attachments: [] },
		{ id: 'later-1', conversationID: 'channel-1', authorExternalID: 'pk-choi', body: '점심 같이 드실 분', postedAt: '2026-10-06T01:30:00.000Z', reactions: [], attachments: [] }
	],
	'dm-park': [
		{ id: 'direct-park', conversationID: 'dm-park', authorExternalID: 'pk-park', body: '자료 보냈어요', postedAt: '2026-10-05T01:00:00.000Z', reactions: [], attachments: [] }
	],
	'dm-choi': [
		{ id: 'direct-choi', conversationID: 'dm-choi', authorExternalID: 'pk-choi', body: '회의실 잡았습니다', postedAt: '2026-10-05T02:00:00.000Z', reactions: [], attachments: [] }
	]
};

class NotificationDestinationFixture extends WorkspaceLoadingFixture {
	readonly marks: Record<string, unknown>[] = [];
	readonly asked: string[] = [];
	isAccountAnswered = true;

	constructor(readonly conversations: Conversation[]) {
		super();
	}

	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		this.asked.push(capability);
		if (capability === 'person.conversations.list') return { status: 200, body: { conversations: this.conversations } };
		if (capability === 'person.people.list') return { status: 200, body: { people } };
		if (capability === 'person.inbox.list') return { status: 200, body: { items: [] } };
		if (capability === 'person.messages.list') return { status: 200, body: { messages: messages[String(body.conversationID)] ?? [], hasMoreBefore: false } };
		if (capability === 'person.read_state.mark') {
			this.marks.push(body);
			return { status: 200, body: {} };
		}
		if (capability === 'person.dm.reveal') {
			const revealed = this.conversations.find((conversation) => conversation.id === body.conversationID);
			if (revealed) revealed.isHidden = false;
			return { status: 200, body: {} };
		}
		return super.host(capability, body);
	}
}

async function prepareMessenger(page: Page, width: number, fixture: NotificationDestinationFixture): Promise<void> {
	await page.setViewportSize({ width, height: 900 });
	await prepareWorkspaceLoading(page, fixture);
	await page.route('http://127.0.0.1:56801/auth/v1/user', (route) => {
		if (!fixture.isAccountAnswered) return route.fulfill({ status: 503, json: { message: 'Fixture account read failure' } });
		return route.fulfill({ json: { id: readerID, email: 'member@example.com', aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {} } });
	});
	await page.route('http://127.0.0.1:56801/rest/v1/member?**', async (route) => {
		if (route.request().url().includes('user_id=eq.')) return route.fallback();
		await route.fulfill({ json: members });
	});
}

const rowNamed = (page: Page, name: string) => page.locator('[data-sidebar="menu-item"]').filter({ hasText: name });

test('a notification for a conversation this device has not listed yet waits for the list and opens it', async ({ page }) => {
	const fixture = new NotificationDestinationFixture([square, withPark]);
	await prepareMessenger(page, 1280, fixture);
	await page.goto('/messenger/?channel=channel-1');
	await expect(page.getByText('점심 같이 드실 분')).toBeVisible();

	fixture.conversations.push(withChoi);
	fixture.hold('person.conversations.list');
	await page.goto('/messenger/?channel=dm-choi&message=direct-choi');
	await expect(rowNamed(page, '박예시')).toBeVisible();
	await expect(rowNamed(page, '최견본')).toHaveCount(0);
	await expect(page).toHaveURL(/channel=dm-choi/);
	fixture.release('person.conversations.list');

	await expect(page.getByText('회의실 잡았습니다')).toBeVisible();
	await expect(page).toHaveURL(/channel=dm-choi/);
});

test('a notification for an unlisted conversation falls back to the last one when the account cannot be read', async ({ page }) => {
	const fixture = new NotificationDestinationFixture([square, withPark]);
	await prepareMessenger(page, 1280, fixture);
	await page.goto('/messenger/?channel=channel-1');
	await expect(page.getByText('점심 같이 드실 분')).toBeVisible();

	fixture.isAccountAnswered = false;
	await page.goto('/messenger/?channel=dm-choi&message=direct-choi');

	await expect(rowNamed(page, '박예시')).toBeVisible();
	await expect(page.getByText('점심 같이 드실 분')).toBeVisible();
});

test('a notification for a hidden direct conversation opens it and brings it back to the list', async ({ page }) => {
	const fixture = new NotificationDestinationFixture([square, { ...withPark, isHidden: true }, withChoi]);
	await prepareMessenger(page, 1280, fixture);
	await page.goto('/messenger/?channel=dm-park&message=direct-park');

	await expect(page.getByText('자료 보냈어요')).toBeVisible();
	await expect(page).toHaveURL(/channel=dm-park/);
	await expect(rowNamed(page, '박예시')).toBeVisible();
	expect(fixture.asked).toContain('person.dm.reveal');
});

test('on a phone a thread reply notification marks its thread read and leaves the channel behind it unread', async ({ page }) => {
	const fixture = new NotificationDestinationFixture([square, withPark]);
	await prepareMessenger(page, 390, fixture);
	await page.goto('/messenger/?channel=channel-1&message=reply-1');

	await expect(page.getByRole('region', { name: '글타래' }).getByText('로그인 화면 고쳤어요')).toBeVisible();
	await expect.poll(() => fixture.marks).toContainEqual({ conversationID: 'channel-1', parentID: 'root-1', messageID: 'reply-1' });
	expect(fixture.marks.filter((mark) => mark.parentID === undefined)).toEqual([]);
});

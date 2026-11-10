import { expect, test, type Page } from '@playwright/test';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from '../shell/workspace-loading-fixture';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

const readerMemberID = '10000000-0000-4000-8000-000000000001';

const conversations = [
	{ id: 'channel-1', name: '광장', kind: 'group', isPrivate: false, participantExternalIDs: ['pk-reader', 'pk-park', 'pk-choi'] }
];

const people = [
	{ externalID: 'pk-reader', name: '이샘플' },
	{ externalID: 'pk-park', name: '박예시' },
	{ externalID: 'pk-choi', name: '최견본' }
];

const members = [
	{ id: readerMemberID, name: '이샘플', email: 'member@example.com', messenger: { buzz: 'pk-reader' }, is_admin: true },
	{ id: '30000000-0000-4000-8000-000000000002', name: '박예시', email: 'park@example.com', messenger: { buzz: 'pk-park' }, is_admin: false },
	{ id: '30000000-0000-4000-8000-000000000003', name: '최견본', email: 'choi@example.com', messenger: { buzz: 'pk-choi' }, is_admin: false }
];

const ownMessage = {
	id: 'own-1',
	conversationID: 'channel-1',
	authorExternalID: 'pk-reader',
	body: '@박예시 회의 자료 올렸어요',
	postedAt: '2026-10-06T01:00:00.000Z',
	mentions: { externalIDs: ['pk-park'], isEveryone: false },
	reactions: [],
	attachments: []
};

class EditMentionsFixture extends WorkspaceLoadingFixture {
	readonly edits: Record<string, unknown>[] = [];

	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		if (capability === 'person.conversations.list') return { status: 200, body: { conversations } };
		if (capability === 'person.people.list') return { status: 200, body: { people } };
		if (capability === 'person.inbox.list') return { status: 200, body: { items: [] } };
		if (capability === 'person.messages.list') return { status: 200, body: { messages: [ownMessage], hasMoreBefore: false } };
		if (capability === 'person.message.edit') {
			this.edits.push(body);
			return { status: 200, body: {} };
		}
		return super.host(capability, body);
	}
}

async function openChannel(page: Page): Promise<EditMentionsFixture> {
	const fixture = new EditMentionsFixture();
	await page.setViewportSize({ width: 1280, height: 900 });
	await prepareWorkspaceLoading(page, fixture);
	await page.route('http://127.0.0.1:56801/rest/v1/member?**', async (route) => {
		if (route.request().url().includes('user_id=eq.')) return route.fallback();
		await route.fulfill({ json: members });
	});
	await page.goto('/messenger/?channel=channel-1');
	return fixture;
}

async function startEditing(page: Page) {
	await page.getByText('회의 자료 올렸어요').click({ button: 'right' });
	await page.getByRole('menuitem', { name: '수정', exact: true }).click();
	return page.getByRole('textbox', { name: '메시지 수정' });
}

test('an edit keeps the people the message already called on and adds the one picked while editing', async ({ page }) => {
	const fixture = await openChannel(page);
	const editor = await startEditing(page);

	await editor.press('End');
	await editor.pressSequentially(' @최');
	await page.getByRole('option', { name: /최견본/ }).click();
	await editor.press('Enter');

	await expect.poll(() => fixture.edits).toHaveLength(1);
	expect(fixture.edits[0]).toMatchObject({
		conversationID: 'channel-1',
		messageID: 'own-1',
		body: '@박예시 회의 자료 올렸어요 @최견본',
		mentions: { externalIDs: ['pk-park', 'pk-choi'], isEveryone: false }
	});
});

test('a person whose name the edit removes is no longer called on', async ({ page }) => {
	const fixture = await openChannel(page);
	const editor = await startEditing(page);

	await editor.fill('회의 자료 올렸어요');
	await editor.press('Enter');

	await expect.poll(() => fixture.edits).toHaveLength(1);
	expect(fixture.edits[0]).toMatchObject({ mentions: { externalIDs: [], isEveryone: false } });
});

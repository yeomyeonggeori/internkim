import { expect, test, type Page } from '@playwright/test';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from '../shell/workspace-loading-fixture';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

type Account = { memberID: string; email: string; channelName: string; messageText: string };

const companyID = '20000000-0000-4000-8000-000000000001';
const firstAccount: Account = { memberID: '10000000-0000-4000-8000-000000000001', email: 'member@example.com', channelName: '샘플광장', messageText: '이전 계정의 대화 내용' };
const secondAccount: Account = { memberID: '10000000-0000-4000-8000-000000000002', email: 'other@example.com', channelName: '새 범위 대화', messageText: '새 계정의 대화 내용' };
const channelID = 'shared-channel';
const otherChannel = { id: 'other-channel', name: '다른 대화', kind: 'group', isPrivate: false, participantExternalIDs: [] };

class AccountChangeFixture extends WorkspaceLoadingFixture {
	account = firstAccount;
	private readonly accountsAsked = new Map<string, Account[]>();

	async waitFor(name: string): Promise<void> {
		const asked = this.accountsAsked.get(name) ?? [];
		asked.push(this.account);
		this.accountsAsked.set(name, asked);
		await super.waitFor(name);
	}

	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		const asking = this.accountsAsked.get(capability)?.shift() ?? this.account;
		if (capability === 'person.conversations.list') {
			return { status: 200, body: { conversations: [{ id: channelID, name: asking.channelName, kind: 'group', isPrivate: false, participantExternalIDs: [] }, otherChannel] } };
		}
		if (capability === 'person.people.list') return { status: 200, body: { people: [] } };
		if (capability === 'person.inbox.list') return { status: 200, body: { items: [] } };
		if (capability === 'person.messages.list') {
			if (body.conversationID !== channelID) return { status: 200, body: { messages: [], hasMoreBefore: false } };
			const message = { id: `message-of-${asking.memberID}`, conversationID: channelID, authorExternalID: 'sample-author', body: asking.messageText, postedAt: '2026-10-06T02:00:00.000Z', reactions: [], attachments: [] };
			return { status: 200, body: { messages: [message], hasMoreBefore: false } };
		}
		return super.host(capability, body);
	}
}

async function openMessenger(page: Page, fixture: AccountChangeFixture): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await prepareWorkspaceLoading(page, fixture);
	await page.route('http://127.0.0.1:56801/auth/v1/user', (route) =>
		route.fulfill({ json: { id: fixture.account.memberID, email: fixture.account.email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {} } })
	);
	await page.route('http://127.0.0.1:56801/rest/v1/member?**', async (route) => {
		if (!route.request().url().includes('user_id=eq.')) return route.fulfill({ json: [] });
		await route.fulfill({ json: { id: fixture.account.memberID, company_id: companyID, is_admin: true, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } });
	});
	await page.goto(`/messenger?channel=${channelID}`);
}

async function signInElsewhereAs(page: Page, fixture: AccountChangeFixture, account: Account): Promise<void> {
	fixture.account = account;
	await page.evaluate(({ memberID, email }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		const session = {
			access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`,
			refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600,
			user: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() }
		};
		localStorage.setItem('sb-127-auth-token', JSON.stringify(session));
		const otherTab = new BroadcastChannel('sb-127-auth-token');
		otherTab.postMessage({ event: 'SIGNED_IN', session });
		otherTab.close();
	}, account);
}

function channelInSidebar(page: Page, name: string) {
	return page.locator('[data-sidebar="menu-button"]').filter({ hasText: name });
}

test('a known conversation survives a refresh, and another account hides it until its own list settles', async ({ page }) => {
	const fixture = new AccountChangeFixture();
	await openMessenger(page, fixture);
	await expect(page.getByText(firstAccount.messageText)).toBeVisible();
	fixture.hold('person.conversations.list');
	await page.reload();
	await expect(page.getByText(firstAccount.messageText)).toBeVisible();
	await expect(channelInSidebar(page, firstAccount.channelName)).toBeVisible();
	await expect(page.locator('[data-messenger-list-skeleton]')).toHaveCount(0);

	await signInElsewhereAs(page, fixture, secondAccount);

	await expect(page.locator('[data-messenger-list-skeleton]').first()).toBeVisible();
	await expect(page.getByText(firstAccount.messageText)).toHaveCount(0);
	await expect(channelInSidebar(page, firstAccount.channelName)).toHaveCount(0);
	fixture.release('person.conversations.list');
	await expect(channelInSidebar(page, secondAccount.channelName)).toBeVisible();
	await expect(page.getByText(secondAccount.messageText)).toBeVisible();
	await expect(page.getByText(firstAccount.messageText)).toHaveCount(0);
});

test('a read the previous account started never reaches the next account', async ({ page }) => {
	const fixture = new AccountChangeFixture();
	await openMessenger(page, fixture);
	await expect(page.getByText(firstAccount.messageText)).toBeVisible();
	fixture.hold('person.messages.list');
	await channelInSidebar(page, otherChannel.name).click();
	await channelInSidebar(page, firstAccount.channelName).click();
	await expect(page).toHaveURL(new RegExp(`channel=${channelID}`));
	await expect(page.getByText(firstAccount.messageText)).toBeVisible();
	await expect.poll(() => fixture.requested.has('person.messages.list')).toBe(true);

	await signInElsewhereAs(page, fixture, secondAccount);
	await expect(page.getByText(firstAccount.messageText)).toHaveCount(0);
	fixture.release('person.messages.list');

	await expect(page.getByText(secondAccount.messageText)).toBeVisible();
	await expect(page.getByText(firstAccount.messageText)).toHaveCount(0);
	const storedMessenger = await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('messenger-')).map((key) => localStorage.getItem(key)).join('\n'));
	expect(storedMessenger).not.toContain(firstAccount.messageText);
});

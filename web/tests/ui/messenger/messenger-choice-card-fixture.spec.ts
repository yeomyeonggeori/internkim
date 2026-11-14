import { expect, test, type Page } from '@playwright/test';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from '../shell/workspace-loading-fixture';
import { everyScreen } from '../support/screen-widths';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

const readerMemberID = '10000000-0000-4000-8000-000000000001';

const conversations = [
	{ id: 'channel-1', name: '광장', kind: 'group', isPrivate: false, participantExternalIDs: ['pk-reader', 'pk-park', 'pk-agent'] }
];

const people = [
	{ externalID: 'pk-reader', name: '이샘플' },
	{ externalID: 'pk-park', name: '박예시' }
];

const members = [
	{ id: readerMemberID, name: '이샘플', email: 'member@example.com', messenger: { buzz: 'pk-reader' }, is_admin: true },
	{ id: '30000000-0000-4000-8000-000000000002', name: '박예시', email: 'park@example.com', messenger: { buzz: 'pk-park' }, is_admin: false }
];

const timeOptions = [
	{ key: 'morning', label: '오전 10시', isRecommended: true },
	{ key: 'afternoon', label: '오후 3시', isRecommended: false }
];

const approvalOptions = [
	{ key: 'approve', label: '', isRecommended: false },
	{ key: 'reject', label: '', isRecommended: false }
];

function agentMessage(id: string, minute: number, fields: Record<string, unknown>) {
	return {
		id,
		conversationID: 'channel-1',
		authorExternalID: 'pk-agent',
		postedAt: `2026-10-06T01:${String(minute).padStart(2, '0')}:00.000Z`,
		reactions: [],
		attachments: [],
		...fields
	};
}

function approvalFor(interactionID: string, targetExternalID: string, resolution?: { key?: string }) {
	return { interactionID, kind: 'approval', targetExternalID, options: approvalOptions, resolution };
}

class ChoiceCardFixture extends WorkspaceLoadingFixture {
	readonly sent: Record<string, unknown>[] = [];
	readerResolution: { key?: string } | undefined;
	isRefusingToSend = false;

	messages() {
		return [
			agentMessage('resolved', 1, {
				body: '어느 회의실로 잡을까요?\n\n1. 3층\n2. 5층',
				ask: {
					interactionID: 'interaction-resolved',
					kind: 'choice',
					targetExternalID: 'pk-reader',
					options: [
						{ key: 'third', label: '3층', isRecommended: false },
						{ key: 'fifth', label: '5층', isRecommended: false }
					],
					resolution: { key: 'fifth' }
				}
			}),
			agentMessage('own-words', 2, {
				body: '자료는 어디에 둘까요?\n\n1. 공유 폴더\n2. 데이터룸',
				ask: {
					interactionID: 'interaction-own-words',
					kind: 'choice',
					targetExternalID: 'pk-reader',
					options: [
						{ key: 'shared', label: '공유 폴더', isRecommended: false },
						{ key: 'dataroom', label: '데이터룸', isRecommended: false }
					],
					resolution: {}
				}
			}),
			agentMessage('failed', 3, { body: '일정을 만들지 못했어요. 캘린더 연결을 확인해 주세요.', isFailure: true }),
			agentMessage('approval-resolved', 4, {
				body: '지난주 회의록을 지울까요?',
				ask: approvalFor('interaction-approval-resolved', 'pk-reader', { key: 'reject' })
			}),
			agentMessage('asks-park', 5, {
				body: '박예시님, 견적서를 거래처에 보낼까요?',
				ask: approvalFor('interaction-park', 'pk-park')
			}),
			agentMessage('approval-reader', 6, {
				body: '박예시님께 이렇게 보낼까요?\n\n> 내일 회의를 10시로 옮겨요',
				ask: approvalFor('interaction-approval-reader', 'pk-reader')
			}),
			agentMessage('asks-reader', 7, {
				body: '주간 회의 시간을 정해야 해요.\n\n언제가 좋을까요?\n\n1. 오전 10시\n2. 오후 3시',
				ask: {
					interactionID: 'interaction-reader',
					kind: 'choice',
					targetExternalID: 'pk-reader',
					options: timeOptions,
					resolution: this.readerResolution
				}
			})
		];
	}

	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		if (capability === 'person.conversations.list') return { status: 200, body: { conversations } };
		if (capability === 'person.people.list') return { status: 200, body: { people, agentExternalID: 'pk-agent' } };
		if (capability === 'person.inbox.list') return { status: 200, body: { items: [] } };
		if (capability === 'person.messages.list') return { status: 200, body: { messages: this.messages(), hasMoreBefore: false } };
		if (capability === 'person.message.send') return this.send(body);
		return super.host(capability, body);
	}

	send(body: Record<string, unknown>): { status: number; body: unknown } {
		if (this.isRefusingToSend) return { status: 503, body: { error: 'Fixture send refused' } };
		this.sent.push(body);
		return { status: 200, body: { ...agentMessage('reply', 8, { body: body.body }), authorExternalID: 'pk-reader' } };
	}
}

async function openChannel(page: Page, width: number): Promise<ChoiceCardFixture> {
	const fixture = new ChoiceCardFixture();
	await page.setViewportSize({ width, height: 900 });
	await prepareWorkspaceLoading(page, fixture);
	await page.route('http://127.0.0.1:56801/rest/v1/member?**', async (route) => {
		if (route.request().url().includes('user_id=eq.')) return route.fallback();
		await route.fulfill({ json: members });
	});
	await page.goto('/messenger/?channel=channel-1');
	return fixture;
}

const messageOf = (page: Page, id: string) => page.locator(`[data-message-id="${id}"]`);
const cardOf = (page: Page, id: string) => messageOf(page, id).locator('[data-slot="choice-card"]');
const looksIn = (page: Page, id: string) =>
	cardOf(page, id).locator('[data-look]').evaluateAll((items) => items.map((item) => item.getAttribute('data-look')));

const shownStates = ['asks-reader', 'approval-reader', 'asks-park', 'resolved', 'approval-resolved', 'own-words', 'failed'];

for (const width of everyScreen) {
	test(`each choice card shows who may answer, and is gone once answered, at ${width}px`, async ({ page }, testInfo) => {
		await openChannel(page, width);

		const asksReader = messageOf(page, 'asks-reader');
		await expect(asksReader.getByText('언제가 좋을까요?')).toBeVisible();
		await expect(asksReader.getByText('1. 오전 10시')).toHaveCount(0);
		await expect(asksReader.getByRole('button', { name: /오전 10시/ })).toContainText('추천');
		await expect(asksReader.getByRole('button', { name: /오후 3시/ })).toBeEnabled();

		const approvalReader = messageOf(page, 'approval-reader');
		await expect(approvalReader.getByRole('button', { name: '승인' })).toBeEnabled();
		await expect(approvalReader.getByRole('button', { name: '거절' })).toBeEnabled();
		const quotedText = approvalReader.locator('blockquote p');
		await expect(quotedText).toHaveText('내일 회의를 10시로 옮겨요');
		expect(
			await quotedText.evaluate((paragraph) => [
				getComputedStyle(paragraph).fontStyle,
				getComputedStyle(paragraph, '::before').content,
				getComputedStyle(paragraph, '::after').content
			])
		).toEqual(['normal', 'none', 'none']);

		const asksPark = messageOf(page, 'asks-park');
		await expect(asksPark.getByRole('button', { name: '승인' })).toBeDisabled();
		await expect(asksPark.getByRole('button', { name: '거절' })).toBeDisabled();

		for (const id of ['resolved', 'approval-resolved', 'own-words']) {
			await expect(cardOf(page, id)).toHaveCount(0);
		}
		await expect(messageOf(page, 'resolved').getByText('어느 회의실로 잡을까요?')).toBeVisible();
		await expect(messageOf(page, 'approval-resolved').getByText('지난주 회의록을 지울까요?')).toBeVisible();

		await expect(messageOf(page, 'failed').getByText('일정을 만들지 못했어요. 캘린더 연결을 확인해 주세요.')).toBeVisible();

		for (const id of shownStates) {
			await messageOf(page, id).screenshot({ path: testInfo.outputPath(`${id}-${width}.png`) });
		}
	});
}

test('pressing an option sends its label as the answer and the card goes away for good', async ({ page }) => {
	const fixture = await openChannel(page, everyScreen[0]);
	const asksReader = messageOf(page, 'asks-reader');

	await asksReader.getByRole('button', { name: /오후 3시/ }).click();

	await expect.poll(() => fixture.sent).toHaveLength(1);
	expect(fixture.sent[0]).toMatchObject({
		conversationID: 'channel-1',
		body: '오후 3시',
		answer: { interactionID: 'interaction-reader', key: 'afternoon' }
	});
	expect(fixture.sent[0].parentID).toBeUndefined();
	await expect(cardOf(page, 'asks-reader')).toHaveCount(0);

	fixture.readerResolution = { key: 'afternoon' };
	await page.reload();
	await expect(messageOf(page, 'asks-reader').getByText('언제가 좋을까요?')).toBeVisible();
	await expect(cardOf(page, 'asks-reader')).toHaveCount(0);
});

test('an approval is answered with the label the reader saw and the key the agent asked with', async ({ page }) => {
	const fixture = await openChannel(page, everyScreen[0]);

	await messageOf(page, 'approval-reader').getByRole('button', { name: '승인' }).click();

	await expect.poll(() => fixture.sent).toHaveLength(1);
	expect(fixture.sent[0]).toMatchObject({ body: '승인', answer: { interactionID: 'interaction-approval-reader', key: 'approve' } });
	await expect(cardOf(page, 'approval-reader')).toHaveCount(0);
});

test('an answer that was not sent says so and can be pressed again', async ({ page }) => {
	const fixture = await openChannel(page, everyScreen[0]);
	fixture.isRefusingToSend = true;

	await messageOf(page, 'asks-reader').getByRole('button', { name: /오후 3시/ }).click();

	await expect(page.getByText('보내지 못했어요')).toBeVisible();
	await expect.poll(() => looksIn(page, 'asks-reader')).toEqual(['pressable', 'pressable']);
	expect(fixture.sent).toHaveLength(0);
});

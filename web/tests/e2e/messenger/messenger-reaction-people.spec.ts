import { expect, test, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'reaction-people-channel';
const voters = ['이샘플', '박예시', '최견본', '김예제', '정테스트'];

async function mockReactions(page: Page) {
	await mockDeviceMessenger(page, { id: 'reaction-reader', email: 'reader@example.com' }, [{ id: channelID, name: '샘플 투표', kind: 'group', myRole: 'member' }], [
		{
			id: 'vote-message', sender: { id: 'vote-author', name: '이샘플' }, text: '숫자로 투표해 주세요', sentAt: '2026-10-06T02:00:00Z',
			reactions: [
				{ emoji: '6️⃣', value: 'six', count: voters.length, reactedByMe: false, people: voters.map((name, index) => ({ id: `voter-${index}`, name })) },
				{ emoji: '2️⃣', value: 'two', count: 1, reactedByMe: false, people: [{ id: 'voter-two', name: '한견본' }] }
			]
		},
		{ id: 'vote-reply', sender: { id: 'voter-two', name: '한견본' }, text: '저는 2번이요', sentAt: '2026-10-06T02:05:00Z', threadRootId: 'vote-message' }
	]);
	await page.route('**/agent/api/person-pictures', route => route.fulfill({ json: {} }));
}

async function holdWithFinger(chip: Locator) {
	const box = await chip.boundingBox();
	const point = { pointerType: 'touch', isPrimary: true, clientX: (box?.x ?? 0) + 4, clientY: (box?.y ?? 0) + 4 };
	await chip.dispatchEvent('pointerdown', point);
	await chip.page().waitForTimeout(900);
	await chip.dispatchEvent('pointerup', point);
	await chip.dispatchEvent('click');
}

async function expectReplyBelowReactions(page: Page) {
	const reactionRow = await page.locator('.message-reactions').first().boundingBox();
	const replyChip = await page.getByRole('button', { name: /1개 답글/ }).boundingBox();
	expect(replyChip?.y ?? 0).toBeGreaterThanOrEqual((reactionRow?.y ?? 0) + (reactionRow?.height ?? 0));
}

test.use({ locale: 'ko-KR', contextOptions: { reducedMotion: 'reduce' } });

test.describe('on a phone', () => {
	test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

	test('holding a reaction opens everyone who reacted instead of the message menu', async ({ page }) => {
		await mockReactions(page);
		await page.goto(`/messenger?channel=${channelID}`);
		const chip = page.getByRole('button', { name: /6️⃣\s*5/ });
		await expect(chip).toBeVisible();
		await expect(page.getByRole('button', { name: '공감한 사람 보기' })).toHaveCount(0);
		await expectReplyBelowReactions(page);

		await holdWithFinger(chip);

		const sheet = page.getByRole('dialog', { name: '공감 6개' });
		await expect(sheet).toBeVisible();
		await expect(sheet.getByRole('tab', { selected: true })).toContainText('5');
		for (const name of voters) await expect(sheet.getByRole('tabpanel').getByText(name, { exact: true })).toBeVisible();
		await expect(page.getByRole('menu')).toHaveCount(0);
		await expect(page.getByText(/눌렀어요/)).toHaveCount(0);
		await expect(chip).toHaveAttribute('aria-pressed', 'false');

		await sheet.getByRole('tab', { name: /전체/ }).click();
		await expect(sheet.getByRole('tabpanel').getByText('한견본', { exact: true })).toBeVisible();
	});
});

test.describe('on a computer', () => {
	test('the people button lists everyone who reacted beside it', async ({ page }) => {
		await mockReactions(page);
		await page.goto(`/messenger?channel=${channelID}`);
		await expectReplyBelowReactions(page);

		await page.getByRole('button', { name: '공감한 사람 보기' }).click();

		const popover = page.getByRole('dialog', { name: '공감 6개' });
		await expect(popover.getByRole('tab', { selected: true })).toContainText('전체');
		for (const name of [...voters, '한견본']) await expect(popover.getByRole('tabpanel').getByText(name, { exact: true })).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(popover).toHaveCount(0);
	});

	test('right-clicking a reaction opens the same list on that reaction, and another reaction switches to it', async ({ page }) => {
		await mockReactions(page);
		await page.goto(`/messenger?channel=${channelID}`);
		await page.getByRole('button', { name: /2️⃣\s*1/ }).click({ button: 'right' });

		const popover = page.getByRole('dialog', { name: '공감 6개' });
		await expect(popover.getByRole('tab', { selected: true })).toContainText('1');
		await expect(popover.getByRole('tabpanel').getByText('한견본', { exact: true })).toBeVisible();
		await expect(page.locator('[data-slot="drawer-content"]')).toHaveCount(0);
		await expect(page.getByRole('menu')).toHaveCount(0);

		await page.getByRole('button', { name: /6️⃣\s*5/ }).click({ button: 'right' });
		await expect(popover.getByRole('tab', { selected: true })).toContainText('5');
	});

	test('hovering a reaction still names who reacted', async ({ page }) => {
		await mockReactions(page);
		await page.goto(`/messenger?channel=${channelID}`);
		await page.getByRole('button', { name: /2️⃣\s*1/ }).hover();

		await expect(page.getByText('한견본님이 눌렀어요')).toBeVisible();
	});
});

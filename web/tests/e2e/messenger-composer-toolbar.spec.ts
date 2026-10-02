import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-composer';
const reader = { id: 'person-reader', email: 'reader@example.com' };

test.describe('messenger composer toolbar', () => {
	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await mockDeviceMessenger(
			page,
			reader,
			[{ id: channelID, name: '입력 채널', kind: 'group', myRole: 'member' }],
			[]
		);
		await page.goto(`/messenger?channel=${channelID}`);
	});

	test('a format button shows the selected words formatted in the composer', async ({ page }) => {
		const composer = page.getByRole('combobox', { name: /메시지/ });
		await composer.fill('안녕 하세요');
		for (let step = 0; step < 3; step++) await composer.press('Shift+ArrowLeft');

		await page.getByRole('button', { name: '서식 표시' }).click();
		await page.getByRole('button', { name: '굵게' }).click();
		await expect(composer.locator('strong')).toHaveText('하세요');
		await expect(page.getByRole('button', { name: '굵게' })).toHaveAttribute('aria-pressed', 'true');

		await composer.press('ControlOrMeta+a');
		await page.getByRole('button', { name: '글머리 목록' }).click();
		await expect(composer.locator('ul > li')).toHaveText('안녕 하세요');
	});

	test('pressing a format button again turns it off for what comes next', async ({ page }) => {
		const composer = page.getByRole('combobox', { name: /메시지/ });
		await page.getByRole('button', { name: '서식 표시' }).click();
		const bold = page.getByRole('button', { name: '굵게' });

		await composer.click();
		await bold.click();
		await composer.pressSequentially('굵게');
		await bold.click();
		await composer.pressSequentially(' 보통');

		await expect(composer.locator('strong')).toHaveText('굵게');
		await expect(composer).toHaveText('굵게 보통');
		await expect(composer).not.toContainText('*');
	});

	test('Enter sends what the composer shows as markdown, and Shift+Enter starts a new line', async ({ page }) => {
		const sent: string[] = [];
		await page.route('**/agent/api/dm**', async (route) => {
			if (route.request().method() === 'POST') {
				const { message } = route.request().postDataJSON() as { message: string };
				sent.push(message);
			}
			await route.fulfill({
				json: { conversationID: channelID, currentUserId: reader.id, messages: [], hasMoreBefore: false, historyCursor: '' }
			});
		});
		const composer = page.getByRole('combobox', { name: /메시지/ });
		await page.getByRole('button', { name: '서식 표시' }).click();

		await composer.click();
		await page.getByRole('button', { name: '굵게' }).click();
		await composer.pressSequentially('굵게');
		await page.getByRole('button', { name: '굵게' }).click();
		await composer.pressSequentially(' 보통');
		await composer.press('Shift+Enter');
		await composer.pressSequentially('둘째 줄');
		await composer.press('Enter');

		await expect.poll(() => sent).toEqual(['**굵게** 보통\n둘째 줄']);
		await expect(composer).toHaveText('');
	});

	test('the format buttons stay hidden until asked for, and the choice survives a reload', async ({ page }) => {
		const bold = page.getByRole('button', { name: '굵게' });
		await expect(bold).toBeHidden();

		await page.getByRole('button', { name: '서식 표시' }).click();
		await expect(bold).toBeVisible();

		await page.reload();
		await expect(bold).toBeVisible();

		await page.getByRole('button', { name: '서식 숨기기' }).click();
		await expect(bold).toBeHidden();
	});

	test('the emoji button puts the chosen emoji at the cursor', async ({ page }) => {
		const composer = page.getByRole('combobox', { name: /메시지/ });
		await composer.fill('좋아요');

		await page.getByRole('button', { name: '이모지 넣기' }).click();
		await page.getByPlaceholder('이모지 검색').fill('thumbs');
		await page.getByRole('option').first().click();

		await expect(composer).toHaveText(/^좋아요\p{Extended_Pictographic}/u);
	});
});

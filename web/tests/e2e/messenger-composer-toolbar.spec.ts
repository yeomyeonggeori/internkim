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

	test('a format button wraps the selected words in markdown', async ({ page }) => {
		const composer = page.getByRole('combobox', { name: /메시지/ });
		await composer.fill('안녕 하세요');
		await composer.evaluate((element: HTMLTextAreaElement) => element.setSelectionRange(0, 2));

		await page.getByRole('button', { name: '서식 표시' }).click();
		await page.getByRole('button', { name: '굵게' }).click();
		await expect(composer).toHaveValue('**안녕** 하세요');

		await composer.evaluate((element: HTMLTextAreaElement) => element.setSelectionRange(0, element.value.length));
		await page.getByRole('button', { name: '글머리 목록' }).click();
		await expect(composer).toHaveValue('- **안녕** 하세요');
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

		await expect(composer).toHaveValue(/^좋아요\p{Extended_Pictographic}/u);
	});
});

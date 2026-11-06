import { readFile } from 'node:fs/promises';
import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const pngSignature = [0x89, 0x50, 0x4e, 0x47];

const messages = [1, 2, 3, 4, 5, 6].map((number) => ({
	id: `message-${number}`,
	sender: author,
	text: `캡처 문장 ${number}`,
	sentAt: `2026-10-07T01:0${number}:00Z`
}));

function row(page: Page, number: number) {
	return page.locator(`[data-message-id="message-${number}"]`);
}

test('a range chosen by clicks after a right click is saved as a PNG', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: 'channel-capture', name: '캡처 채널', kind: 'group', myRole: 'member' }],
		messages
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-capture');

	await row(page, 3).click({ button: 'right' });
	await page.getByRole('menuitem', { name: '대화 캡처' }).click();

	const bar = page.getByRole('toolbar', { name: '대화 캡처' });
	await expect(bar).toContainText('캡처할 메시지를 누르세요');
	await expect(page.getByRole('button', { name: '파일 첨부' })).toBeHidden();

	await row(page, 2).click();
	await expect(bar).toContainText('메시지 1개');
	await row(page, 5).click();
	await expect(bar).toContainText('메시지 4개');
	await row(page, 3).click();
	await expect(bar).toContainText('메시지 3개');
	await expect(page.getByRole('menuitem')).toHaveCount(0);

	const download = page.waitForEvent('download');
	await bar.getByRole('button', { name: '저장' }).click();
	const saved = await download;
	expect(saved.suggestedFilename()).toMatch(/\.png$/);
	const bytes = await readFile(await saved.path());
	expect([...bytes.subarray(0, 4)]).toEqual(pngSignature);

	await expect(bar).toBeHidden();
});

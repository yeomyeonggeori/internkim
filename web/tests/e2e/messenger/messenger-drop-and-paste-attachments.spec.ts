import { expect, test, type JSHandle, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-attach';
const reader = { id: 'person-reader', email: 'reader@example.com' };
const onePixelPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=';

type Carried = { files?: { name: string; type: string; base64: string }[]; text?: string };

function carrying(page: Page, carried: Carried): Promise<JSHandle<DataTransfer>> {
	return page.evaluateHandle(({ files, text }) => {
		const transfer = new DataTransfer();
		for (const file of files ?? []) {
			const bytes = Uint8Array.from(atob(file.base64), (character) => character.charCodeAt(0));
			transfer.items.add(new File([bytes], file.name, { type: file.type }));
		}
		if (text) transfer.setData('text/plain', text);
		return transfer;
	}, carried);
}

function picture(name: string) {
	return { name, type: 'image/png', base64: onePixelPNG };
}

async function dragOnto(target: Locator, transfer: JSHandle<DataTransfer>, events: string[]): Promise<void> {
	for (const type of events) await target.dispatchEvent(type, { dataTransfer: transfer });
}

async function paste(composer: Locator, transfer: JSHandle<DataTransfer>): Promise<void> {
	await composer.evaluate((element, clipboardData) => {
		element.dispatchEvent(new ClipboardEvent('paste', { clipboardData, bubbles: true, cancelable: true }));
	}, transfer);
}

test.describe('messenger attachments by dropping and pasting', () => {
	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await mockDeviceMessenger(
			page,
			reader,
			[{ id: channelID, name: '첨부 채널', kind: 'group', myRole: 'member' }],
			[]
		);
		await page.goto(`/messenger?channel=${channelID}`);
		await expect(page.getByRole('combobox', { name: /메시지/ })).toBeVisible();
	});

	test('dragging files over the conversation covers it, and dropping them attaches each inside the composer', async ({ page }) => {
		const conversation = page.locator('[style*="--dock-height"]');
		const files = await carrying(page, { files: [picture('first.png'), picture('second.png')] });

		await dragOnto(conversation, files, ['dragenter', 'dragover']);
		await expect(page.getByText('여기에 놓으면 첨부돼요')).toBeVisible();

		await dragOnto(conversation, files, ['drop']);
		await expect(page.getByText('여기에 놓으면 첨부돼요')).toBeHidden();
		const composerBox = page.locator('.channel-composer [data-slot="input-group"]');
		await expect(composerBox.getByText('first.png')).toBeVisible();
		await expect(composerBox.getByText('second.png')).toBeVisible();
		await expect(composerBox.getByRole('button', { name: '첨부 제거' })).toHaveCount(2);
	});

	test('leaving the conversation without dropping takes the cover away', async ({ page }) => {
		const conversation = page.locator('[style*="--dock-height"]');
		const files = await carrying(page, { files: [picture('first.png')] });

		await dragOnto(conversation, files, ['dragenter']);
		await expect(page.getByText('여기에 놓으면 첨부돼요')).toBeVisible();
		await dragOnto(conversation, files, ['dragleave']);
		await expect(page.getByText('여기에 놓으면 첨부돼요')).toBeHidden();
	});

	test('dragging words never covers the conversation', async ({ page }) => {
		const conversation = page.locator('[style*="--dock-height"]');
		const words = await carrying(page, { text: '끌어온 글자' });

		await dragOnto(conversation, words, ['dragenter', 'dragover']);
		await expect(page.getByText('여기에 놓으면 첨부돼요')).toBeHidden();
	});

	test('pasting a picture attaches it, and pasting words writes the words', async ({ page }) => {
		const composer = page.getByRole('combobox', { name: /메시지/ });
		await composer.click();

		await paste(composer, await carrying(page, { files: [picture('screenshot.png')] }));
		await expect(page.locator('.channel-composer').getByText('screenshot.png')).toBeVisible();
		await expect(composer).toHaveText('');

		await paste(composer, await carrying(page, { text: '붙여넣은 글자', files: [picture('rendered.png')] }));
		await expect(composer).toHaveText('붙여넣은 글자');
		await expect(page.locator('.channel-composer').getByText('rendered.png')).toHaveCount(0);
	});
});

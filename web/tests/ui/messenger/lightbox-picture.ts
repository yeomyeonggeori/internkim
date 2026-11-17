import { expect, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'lightbox-picture-channel';
const picture = '<svg xmlns="http://www.w3.org/2000/svg" width="540" height="960"><rect width="540" height="960" fill="#466baa"/></svg>';
const pictureSource = `data:image/svg+xml,${encodeURIComponent(picture)}`;

export async function openLightboxPicture(page: Page): Promise<Locator> {
	await mockDeviceMessenger(page, { id: 'picture-reader', email: 'reader@example.com' }, [{ id: channelID, name: '사진 대화', kind: 'group', myRole: 'member' }], [
		{
			id: 'lightbox-picture', sender: { id: 'picture-author', name: '이샘플' }, text: '', sentAt: '2026-10-06T02:00:00Z',
			attachments: [{ kind: 'image', url: pictureSource, source: pictureSource, filename: 'sample-photo.svg', widthPixels: 540, heightPixels: 960 }]
		}
	]);
	await page.route('**/agent/api/person-pictures', route => route.fulfill({ json: {} }));
	await page.route('**/agent/api/custom-emoji', route => route.fulfill({ json: { emojis: [] } }));
	await page.goto(`/messenger?channel=${channelID}`);
	await page.getByRole('button', { name: 'sample-photo.svg', exact: true }).click();
	const frame = page.getByRole('dialog').locator('[data-loading-image]');
	await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
	return frame;
}

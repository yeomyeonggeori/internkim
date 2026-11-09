import { readFileSync } from 'node:fs';
import { expect, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-sending-video';
const reader = { id: 'person-reader', email: 'reader@example.com' };

export type CarriedAttachment = { filename: string; contentType: string; contentBase64: string };

export async function openVideoChannel(page: Page): Promise<{ sent: Promise<CarriedAttachment> }> {
	await page.setViewportSize({ width: 1280, height: 800 });
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '영상 채널', kind: 'group', myRole: 'member' }], []);
	let resolveSent: (attachment: CarriedAttachment) => void = () => {};
	const sent = new Promise<CarriedAttachment>((resolve) => (resolveSent = resolve));
	await page.route('**/agent/api/dm**', async (route) => {
		if (route.request().method() === 'POST') {
			const body: { attachments: CarriedAttachment[] } = route.request().postDataJSON();
			resolveSent(body.attachments[0]);
		}
		await route.fulfill({
			json: { conversationID: channelID, currentUserId: reader.id, messages: [], hasMoreBefore: false, historyCursor: '' }
		});
	});
	await page.goto(`/messenger?channel=${channelID}`);
	return { sent };
}

export async function attachFile(page: Page, name: string, mimeType: string, buffer: Buffer): Promise<void> {
	await page.locator('input[type="file"]').first().setInputFiles([{ name, mimeType, buffer }]);
}

export async function sendFixtureVideo(page: Page, name: string, mimeType: string): Promise<CarriedAttachment> {
	const { sent } = await openVideoChannel(page);
	await attachFile(page, name, mimeType, readFileSync(new URL(`./fixtures/${name}`, import.meta.url)));
	await page.getByRole('button', { name: '보내기' }).click();
	return sent;
}

function topLevelBoxesOf(bytes: Buffer): string[] {
	const boxes: string[] = [];
	let offset = 0;
	while (offset + 8 <= bytes.length) {
		const size = bytes.readUInt32BE(offset);
		boxes.push(bytes.toString('latin1', offset + 4, offset + 8));
		if (size < 8) break;
		offset += size;
	}
	return boxes;
}

export function expectRelayVideo(attachment: CarriedAttachment): void {
	const bytes = Buffer.from(attachment.contentBase64, 'base64');
	const boxes = topLevelBoxesOf(bytes);
	expect(attachment.contentType).toBe('video/mp4');
	expect(attachment.filename.endsWith('.mp4')).toBe(true);
	expect(bytes.toString('latin1', 8, 12)).not.toBe('qt  ');
	expect(boxes.indexOf('moov')).toBeGreaterThan(-1);
	expect(boxes.indexOf('moov')).toBeLessThan(boxes.indexOf('mdat'));
	expect(bytes.includes(Buffer.from('avc1'))).toBe(true);
	expect(bytes.includes(Buffer.from('hvc1'))).toBe(false);
	expect(bytes.includes(Buffer.from('mp4a'))).toBe(true);
}

import { describe, expect, test } from 'bun:test';
import { largestNotificationPictureBytes, notificationPicturePathOf, type NotificationPictureSources } from './notification-picture';

const digest = 'a'.repeat(64);
const keptPath = `43000000-0000-0000-0000-0000000000a0/shared/transfer/${digest}`;
const picture = { pictureDigest: digest, pictureContentType: 'image/png' };

function sourcesWith(keptSizeOf: (path: string) => Promise<number | null>, reported: string[] = []): NotificationPictureSources {
	return {
		pathOf: (named) => `43000000-0000-0000-0000-0000000000a0/shared/transfer/${named}`,
		keptSizeOf,
		report: (line) => reported.push(line),
		timeLimitMilliseconds: 20
	};
}

const keptAtSize = (sizeBytes: number | null) => sourcesWith(async () => sizeBytes);

describe('notificationPicturePathOf', () => {
	test('a kept picture of a showable kind and size is shown', async () => {
		for (const pictureContentType of ['image/jpeg', 'image/png', 'image/gif']) {
			expect(await notificationPicturePathOf({ ...picture, pictureContentType }, keptAtSize(2048))).toBe(keptPath);
		}
	});

	test('a picture exactly at the ceiling is still shown', async () => {
		expect(await notificationPicturePathOf(picture, keptAtSize(largestNotificationPictureBytes))).toBe(keptPath);
	});

	test('a message with no picture shows none', async () => {
		expect(await notificationPicturePathOf({ pictureDigest: '', pictureContentType: '' }, keptAtSize(2048))).toBe('');
	});

	test('a kind a notification cannot draw is not shown', async () => {
		for (const pictureContentType of ['image/webp', 'image/svg+xml', 'video/mp4', 'IMAGE/PNG', 'image/png; charset=binary']) {
			expect(await notificationPicturePathOf({ ...picture, pictureContentType }, keptAtSize(2048))).toBe('');
		}
	});

	test('a digest that is not 64 lowercase hexadecimal characters is not looked up', async () => {
		let lookedUp = 0;
		const sources = sourcesWith(async () => {
			lookedUp += 1;
			return 2048;
		});
		for (const pictureDigest of ['A'.repeat(64), 'a'.repeat(63), 'a'.repeat(65), `${'a'.repeat(62)}/x`, 'g'.repeat(64)]) {
			expect(await notificationPicturePathOf({ ...picture, pictureDigest }, sources)).toBe('');
		}
		expect(lookedUp).toBe(0);
	});

	test('a picture the company has not kept is not shown', async () => {
		expect(await notificationPicturePathOf(picture, keptAtSize(null))).toBe('');
	});

	test('a picture over the ceiling is not shown', async () => {
		expect(await notificationPicturePathOf(picture, keptAtSize(largestNotificationPictureBytes + 1))).toBe('');
	});

	test('a lookup that fails reports one line and shows nothing', async () => {
		const reported: string[] = [];
		const sources = sourcesWith(async () => {
			throw new Error('the store answered 500');
		}, reported);

		expect(await notificationPicturePathOf(picture, sources)).toBe('');
		expect(reported).toHaveLength(1);
		expect(reported[0]).toContain('the store answered 500');
	});

	test('a lookup that does not answer in time reports one line and shows nothing', async () => {
		const reported: string[] = [];
		const sources = sourcesWith(() => new Promise<number | null>(() => {}), reported);

		expect(await notificationPicturePathOf(picture, sources)).toBe('');
		expect(reported).toHaveLength(1);
		expect(reported[0]).toContain('no answer within 20ms');
	});
});

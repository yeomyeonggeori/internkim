import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

test.use({ locale: 'ko-KR', colorScheme: 'dark', contextOptions: { reducedMotion: 'reduce' } });

for (const width of [1280, 390, 320]) {
	test(`shared avatar overflow matches its avatars and stays on top at ${width}px`, async ({ page }, testInfo) => {
		await page.setViewportSize({ width, height: 1000 });
		await mockDeviceMessenger(page, { id: 'sample-reader', email: 'reader@example.com' }, [], []);
		await page.goto('/attendance/stack-preview');
		await expect(page.getByTestId('stack-sizing-small-1')).toBeVisible();
		await page.evaluate(() => document.fonts.ready);
		for (const size of ['small', 'default', 'large']) {
			for (const remaining of [1, 12, 123]) {
				const stack = page.getByTestId(`stack-sizing-${size}-${remaining}`);
				const avatar = stack.locator('[data-slot="avatar"]').first();
				const overflow = stack.getByTestId('avatar-stack-overflow');
				await expect(overflow).toHaveText(`+${remaining}`);
				const avatarBox = await avatar.boundingBox();
				const countBox = await overflow.boundingBox();
				if (!avatarBox || !countBox) throw new Error('The shared avatar stack has no layout box');
				expect(countBox.width).toBe(avatarBox.width);
				expect(countBox.height).toBe(avatarBox.height);
				expect(countBox.y).toBe(avatarBox.y);
				const textBounds = await overflow.evaluate(element => {
					const range = document.createRange();
					range.selectNodeContents(element);
					const bounds = range.getBoundingClientRect();
					return { width: bounds.width, height: bounds.height };
				});
				expect(textBounds.width).toBeLessThan(countBox.width - 2);
				expect(textBounds.height).toBeLessThan(countBox.height - 2);
				expect(await overflow.evaluate(element => {
					const bounds = element.getBoundingClientRect();
					return document.elementFromPoint(bounds.left + 2, bounds.top + bounds.height / 2)?.closest('[data-slot="avatar-group-count"]') === element;
				})).toBe(true);
			}
		}
		await expect(page.getByTestId('stack-none').getByTestId('avatar-stack-overflow')).toHaveCount(0);
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		await page.screenshot({ path: testInfo.outputPath(`avatar-stack-${width}.png`), fullPage: true });
	});
}

test('a single reply chip contains only the reply author, while self replies still include the root author', async ({ page }, testInfo) => {
	const reader = { id: 'sample-reader', name: '이샘플', email: 'reader@example.com', avatarURL: 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="32" height="32"%3E%3Cpath fill="royalblue" d="M0 0h32v32H0z"/%3E%3C/svg%3E' };
	const author = { id: 'sample-author', name: '박예시', email: 'author@example.com', avatarURL: 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="32" height="32"%3E%3Cpath fill="tomato" d="M0 0h32v32H0z"/%3E%3C/svg%3E' };
	const messages = [
		{ id: 'root', sender: author, text: '다른 사람이 답글을 남긴 원글', sentAt: '2026-10-06T03:00:00Z' },
		{ id: 'reply', threadRootId: 'root', sender: reader, text: '첫 답글', sentAt: '2026-10-06T03:01:00Z' },
		{ id: 'self-root', sender: author, text: '작성자가 답글을 남긴 원글', sentAt: '2026-10-06T03:02:00Z' },
		{ id: 'self-reply', threadRootId: 'self-root', sender: author, text: '직접 남긴 답글', sentAt: '2026-10-06T03:03:00Z' }
	];
	await mockDeviceMessenger(page, reader, [{ id: 'sample-channel', name: '샘플 채널', kind: 'group' }], messages);
	await page.goto('/messenger?channel=sample-channel');
	const replyChips = page.getByRole('button', { name: /1개 답글/ });
	await expect(replyChips).toHaveCount(2);
	for (const chip of await replyChips.all()) await expect(chip.locator('[data-slot="avatar"]')).toHaveCount(1);
	const otherReplyChip = page.locator('[data-slot="message"]').filter({ hasText: messages[0].text }).getByRole('button', { name: /1개 답글/ });
	const selfReplyChip = page.locator('[data-slot="message"]').filter({ hasText: messages[2].text }).getByRole('button', { name: /1개 답글/ });
	await expect(otherReplyChip.locator('img')).toHaveAttribute('alt', reader.name);
	await expect(selfReplyChip.locator('img')).toHaveAttribute('alt', author.name);
	await otherReplyChip.click();
	await expect(page.getByRole('dialog', { name: '글타래' }).getByText('첫 답글', { exact: true })).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog', { name: '글타래' })).toHaveCount(0);
	await page.screenshot({ path: testInfo.outputPath('reply-avatars.png') });
});

test('the messenger overflow counts only reply authors and matches the small reply avatars', async ({ page }, testInfo) => {
	const author = { id: 'sample-root-author', name: '이샘플', email: 'root@example.com' };
	const replyAuthors = Array.from({ length: 4 }, (_, index) => ({ id: `sample-reply-author-${index}`, name: `샘플 ${index}`, email: `reply-${index}@example.com` }));
	const root = { id: 'root-with-many-replies', sender: author, text: '답글 작성자 네 명의 원글', sentAt: '2026-10-06T03:00:00Z' };
	const replies = Array.from({ length: 13 }, (_, index) => ({
		id: `reply-${index}`, threadRootId: root.id, sender: replyAuthors[index % replyAuthors.length], text: `샘플 답글 ${index}`,
		sentAt: `2026-10-06T03:${String(index + 1).padStart(2, '0')}:00Z`
	}));
	await mockDeviceMessenger(page, author, [{ id: 'sample-overflow-channel', name: '샘플 채널', kind: 'group' }], [root, ...replies]);
	await page.goto('/messenger?channel=sample-overflow-channel');
	const chip = page.getByRole('button', { name: /13개 답글/ });
	await expect(chip).toBeVisible();
	await expect(chip.locator('[data-slot="avatar"]')).toHaveCount(3);
	const overflow = chip.getByTestId('avatar-stack-overflow');
	await expect(overflow).toHaveText('+1');
	const avatarBox = await chip.locator('[data-slot="avatar"]').first().boundingBox();
	const countBox = await overflow.boundingBox();
	if (!avatarBox || !countBox) throw new Error('The reply avatars have no layout box');
	expect(avatarBox.width).toBe(20);
	expect(countBox.width).toBe(avatarBox.width);
	expect(countBox.height).toBe(avatarBox.height);
	await chip.screenshot({ path: testInfo.outputPath('messenger-avatar-overflow.png') });
});

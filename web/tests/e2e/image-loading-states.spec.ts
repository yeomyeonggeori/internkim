import { expect, test, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'image-loading-channel';
const reader = { id: 'image-reader', email: 'reader@example.com' };
const author = { id: 'image-author', name: '이샘플' };
const pictureURL = '/image-loading-fixture/portrait.svg';
const picture = '<svg xmlns="http://www.w3.org/2000/svg" width="540" height="960" viewBox="0 0 540 960"><rect width="540" height="960" fill="#777d76"/><rect y="560" width="540" height="400" fill="#d4cfc4"/><rect x="120" y="250" width="300" height="430" rx="16" fill="#466baa"/><circle cx="270" cy="450" r="90" fill="#b7cde5"/><path d="M80 840L230 570L330 635L210 960H80" fill="#c99f81"/></svg>';
const cachedPicture = `data:image/svg+xml,${encodeURIComponent(picture)}`;

function gate() {
	let release = () => {};
	const promise = new Promise<void>(resolve => { release = resolve; });
	return { promise, release };
}

async function capture(page: Page, scene: string) {
	const directory = process.env.LOADING_EVIDENCE_DIRECTORY;
	if (!directory) return;
	await mkdir(directory, { recursive: true });
	await page.screenshot({ path: `${directory}/${scene}.png`, animations: 'disabled' });
}

async function mockChannel(page: Page, source = pictureURL, dimensions: { widthPixels?: number; heightPixels?: number } = { widthPixels: 540, heightPixels: 960 }) {
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '사진 대화', kind: 'group', myRole: 'member' }], [
		{
			id: 'image-message', sender: author, text: '', sentAt: '2026-10-06T02:00:00Z',
			attachments: [{ kind: 'image', url: source, source, filename: 'sample-photo.svg', ...dimensions }],
			thread: { replyCount: 1, lastReplyAt: '2026-10-06T02:01:00Z', participants: [reader] }
		},
		{ id: 'image-reply', threadRootId: 'image-message', sender: reader, text: '사진 확인했어요', sentAt: '2026-10-06T02:01:00Z' }
	]);
	await page.route('**/agent/api/person-pictures', route => route.fulfill({ json: {} }));
	await page.route('**/agent/api/custom-emoji', route => route.fulfill({ json: { emojis: [] } }));
}

test.use({ locale: 'ko-KR', colorScheme: 'dark', contextOptions: { reducedMotion: 'reduce' } });

for (const width of [1280, 390, 320]) {
	test(`thread picture has a stable skeleton until pixels arrive at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		await page.clock.setFixedTime(new Date('2026-10-06T03:00:00Z'));
		await mockChannel(page);
		const pending = gate();
		await page.route(`**${pictureURL}`, async route => {
			await pending.promise;
			await route.fulfill({ contentType: 'image/svg+xml', body: picture });
		});
		try {
			await page.goto(`/messenger?channel=${channelID}`);
			await page.getByRole('button', { name: /1개 답글/ }).click();
			const thread = page.getByRole('dialog', { name: '글타래' });
			const frame = thread.locator('[data-loading-image]');
			await expect(frame).toHaveAttribute('data-loading-image', 'loading');
			await thread.evaluate(element => Promise.all(element.getAnimations({ subtree: true }).map(animation => animation.finished)));
			await expect(frame.getByRole('status', { name: '이미지를 불러오는 중' })).toBeVisible();
			await expect(frame.locator('[data-slot="skeleton"]')).toBeVisible();
			expect(await frame.locator('[data-slot="skeleton"]').evaluate(element => getComputedStyle(element).animationName)).toBe('none');
			const before = await frame.boundingBox();
			if (!before) throw new Error('The loading image has no reserved frame');
			expect(before.width).toBeGreaterThan(100);
			expect(before.height).toBeGreaterThan(180);
			expect(before.width / before.height).toBeCloseTo(540 / 960, 2);
			await capture(page, `image-thread-${width}-loading`);
			pending.release();
			await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
			await expect(frame.locator('[data-slot="skeleton"]')).toHaveCount(0);
			await expect(frame.getByRole('img', { name: 'sample-photo.svg' })).toBeVisible();
			const after = await frame.boundingBox();
			if (!after) throw new Error('The loaded image has no frame');
			expect(after.width).toBeCloseTo(before.width, 1);
			expect(after.height).toBeCloseTo(before.height, 1);
			expect(after.x).toBeCloseTo(before.x, 1);
			expect(after.y).toBeCloseTo(before.y, 1);
			const timestamp = await thread.locator('[data-message-id="image-message"] time').boundingBox();
			if (!timestamp) throw new Error('The picture timestamp has no layout box');
			expect(timestamp.x).toBeGreaterThanOrEqual(after.x + after.width);
			if (process.env.LOADING_EVIDENCE_DIRECTORY) {
				await writeFile(`${process.env.LOADING_EVIDENCE_DIRECTORY}/image-thread-${width}-geometry.json`, JSON.stringify({ before, after, timestamp }, null, 2));
			}
			await capture(page, `image-thread-${width}-loaded`);
			expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
		} finally {
			pending.release();
		}
	});
}

test('a failed image settles to a visible fallback without a perpetual skeleton', async ({ page }) => {
	await mockChannel(page);
	await page.route(`**${pictureURL}`, route => route.fulfill({ status: 404, body: 'Missing image' }));
	await page.goto(`/messenger?channel=${channelID}`);
	const frame = page.locator('[data-loading-image]').first();
	await expect(frame).toHaveAttribute('data-loading-image', 'error');
	await expect(frame.getByRole('img', { name: 'sample-photo.svg: 이미지를 불러올 수 없어요' })).toBeVisible();
	await expect(frame.locator('[data-slot="skeleton"]')).toHaveCount(0);
	await expect(frame).toHaveAttribute('aria-busy', 'false');
});

const unknownSizeCases: { name: string; dimensions: { widthPixels?: number; heightPixels?: number } }[] = [
	{ name: 'missing', dimensions: {} },
	{ name: 'partial', dimensions: { widthPixels: 540 } },
	{ name: 'invalid', dimensions: { widthPixels: 0, heightPixels: -960 } }
];

for (const { name, dimensions } of unknownSizeCases) {
	test(`${name} photo dimensions use a square until the natural portrait ratio is known`, async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 900 });
		await mockChannel(page, pictureURL, dimensions);
		const pending = gate();
		await page.route(`**${pictureURL}`, async route => {
			await pending.promise;
			await route.fulfill({ contentType: 'image/svg+xml', body: picture });
		});
		try {
			await page.goto(`/messenger?channel=${channelID}`);
			await page.getByRole('button', { name: /1개 답글/ }).click();
			const thread = page.getByRole('dialog', { name: '글타래' });
			const frame = thread.locator('[data-loading-image]');
			await expect(frame).toHaveAttribute('data-loading-image', 'loading');
			await thread.evaluate(element => Promise.all(element.getAnimations({ subtree: true }).map(animation => animation.finished)));
			const before = await frame.boundingBox();
			if (!before) throw new Error('The unknown-size photo has no loading frame');
			expect(before.width / before.height).toBeCloseTo(1, 3);
			await capture(page, `image-size-${name}-loading`);
			pending.release();
			await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
			const after = await frame.boundingBox();
			if (!after) throw new Error('The decoded photo has no frame');
			expect(after.width / after.height).toBeCloseTo(540 / 960, 3);
			await expect(frame.locator('img')).toHaveCSS('object-fit', 'contain');
			await capture(page, `image-size-${name}-loaded`);
			if (process.env.LOADING_EVIDENCE_DIRECTORY) {
				await writeFile(`${process.env.LOADING_EVIDENCE_DIRECTORY}/image-size-${name}-geometry.json`, JSON.stringify({ before, after, metadata: dimensions, naturalWidth: 540, naturalHeight: 960 }, null, 2));
			}
		} finally { pending.release(); }
	});
}

test('known landscape metadata reserves its actual ratio before the image arrives', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	await mockChannel(page, pictureURL, { widthPixels: 960, heightPixels: 540 });
	const pending = gate();
	await page.route(`**${pictureURL}`, async route => {
		await pending.promise;
		await route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="960" height="540"><rect width="960" height="540" fill="#466baa"/></svg>' });
	});
	try {
		await page.goto(`/messenger?channel=${channelID}`);
		const frame = page.locator('[data-loading-image]').first();
		await expect(frame).toHaveAttribute('data-loading-image', 'loading');
		const before = await frame.boundingBox();
		if (!before) throw new Error('The landscape photo has no loading frame');
		expect(before.width / before.height).toBeCloseTo(960 / 540, 3);
		pending.release();
		await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
		const after = await frame.boundingBox();
		if (!after) throw new Error('The loaded landscape photo has no frame');
		expect(after.width).toBeCloseTo(before.width, 1);
		expect(after.height).toBeCloseTo(before.height, 1);
	} finally { pending.release(); }
});

test('the lightbox keeps attachment proportions while its image is pending', async ({ page }) => {
	await mockChannel(page);
	const pending = gate();
	await page.route(`**${pictureURL}`, async route => {
		await pending.promise;
		await route.fulfill({ contentType: 'image/svg+xml', body: picture });
	});
	try {
		await page.goto(`/messenger?channel=${channelID}`);
		await page.getByRole('button', { name: 'sample-photo.svg', exact: true }).click();
		const lightbox = page.getByRole('dialog');
		const frame = lightbox.locator('[data-loading-image]');
		await expect(frame).toHaveAttribute('data-loading-image', 'loading');
		const before = await frame.boundingBox();
		if (!before) throw new Error('The lightbox image has no frame');
		expect(before.width / before.height).toBeCloseTo(540 / 960, 2);
		pending.release();
		await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
		const after = await frame.boundingBox();
		if (!after) throw new Error('The loaded lightbox image has no frame');
		expect(after.width).toBeCloseTo(before.width, 1);
		expect(after.height).toBeCloseTo(before.height, 1);
		await lightbox.getByRole('button', { name: '이미지 닫기' }).click({ position: { x: 5, y: 5 } });
		await expect(lightbox).toHaveCount(0);
	} finally {
		pending.release();
	}
});

test('a decoded cached image is visible at the first paint when reopened', async ({ page }) => {
	await mockChannel(page, cachedPicture);
	await page.goto(`/messenger?channel=${channelID}`);
	await expect(page.locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'loaded');
	await page.evaluate(async (source) => {
		const image = new Image();
		image.src = source;
		await image.decode();
	}, cachedPicture);
	const firstPaint = await page.getByRole('button', { name: /1개 답글/ }).evaluate(element => {
		if (!(element instanceof HTMLButtonElement)) throw new Error('The reply trigger is not a button');
		element.click();
		return new Promise<string | null>(resolve => {
			requestAnimationFrame(() => resolve(document.querySelector('[role="dialog"] [data-loading-image]')?.getAttribute('data-loading-image') ?? null));
		});
	});
	expect(firstPaint).toBe('loaded');
	await expect(page.getByRole('dialog', { name: '글타래' }).locator('[data-loading-image] [data-slot="skeleton"]')).toHaveCount(0);
});

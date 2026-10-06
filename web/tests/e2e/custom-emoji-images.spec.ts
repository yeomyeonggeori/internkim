import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'custom-emoji-channel';
const imageURL = '/image-loading-fixture/custom-emoji.svg';
const picture = '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="#6754cf"/><path d="M8 16l5 5L25 9" fill="none" stroke="white" stroke-width="4"/></svg>';

async function mockReactions(page: Page, resolvedImage = imageURL) {
	await mockDeviceMessenger(page, { id: 'reaction-reader', email: 'reader@example.com' }, [{ id: channelID, name: '샘플 반응', kind: 'group', myRole: 'member' }], [{
		id: 'reaction-message', sender: { id: 'reaction-author', name: '이샘플' }, text: '사용자 지정 이모지 확인', sentAt: '2026-10-06T02:00:00Z',
		reactions: [{ emoji: 'sample_custom', value: 'sample_custom', imageURL: '/image-loading-fixture/outdated.svg', count: 3, reactedByMe: false, people: [{ id: 'sample-person', name: '박예시' }] }]
	}]);
	await page.route('**/agent/api/person-pictures', route => route.fulfill({ json: {} }));
	await page.route('**/agent/api/custom-emoji', route => route.fulfill({ json: { emoji: [{ name: 'sample_custom', url: resolvedImage }] } }));
	await page.route('**/image-loading-fixture/outdated.svg', route => route.fulfill({ status: 404, body: 'Outdated image' }));
}

test.use({ locale: 'ko-KR', contextOptions: { reducedMotion: 'reduce' } });

test('the reaction and tooltip share the resolved image with a fixed small loading frame', async ({ page }) => {
	await mockReactions(page);
	let release = () => {};
	const pending = new Promise<void>(resolve => { release = resolve; });
	await page.route(`**${imageURL}`, async route => {
		await pending;
		await route.fulfill({ contentType: 'image/svg+xml', body: picture });
	});
	try {
		await page.goto(`/messenger?channel=${channelID}`);
		const reaction = page.getByRole('button', { name: ':sample_custom: 3', exact: true });
		const frame = reaction.locator('[data-loading-image]');
		await expect(frame).toHaveAttribute('data-loading-image', 'loading');
		await expect(frame.locator('img')).toHaveAttribute('src', imageURL);
		const before = await frame.boundingBox();
		expect(before?.width).toBe(16);
		expect(before?.height).toBe(16);
		await reaction.hover();
		const tooltip = page.locator('[data-slot="tooltip-content"]').filter({ hasText: '박예시님이 눌렀어요' });
		await expect(tooltip.locator('img')).toHaveAttribute('src', imageURL);
		if (process.env.LOADING_EVIDENCE_DIRECTORY) await page.screenshot({ path: `${process.env.LOADING_EVIDENCE_DIRECTORY}/custom-emoji-loading.png`, animations: 'disabled' });
		release();
		await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
		await expect(tooltip.locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'loaded');
		const after = await frame.boundingBox();
		expect(after?.width).toBe(before?.width);
		expect(after?.height).toBe(before?.height);
		if (process.env.LOADING_EVIDENCE_DIRECTORY) await page.screenshot({ path: `${process.env.LOADING_EVIDENCE_DIRECTORY}/custom-emoji-loaded.png`, animations: 'disabled' });
	} finally { release(); }
});

test('failed custom emoji show their shortcode in a read-only conversation', async ({ page }) => {
	await mockReactions(page);
	let mutationRequests = 0;
	page.on('request', request => { if (request.method() !== 'GET' && request.url().includes('/agent/api/')) mutationRequests += 1; });
	await page.route(`**${imageURL}`, route => route.fulfill({ status: 404, body: 'Missing custom emoji' }));
	await page.goto(`/messenger?channel=${channelID}`);
	const reaction = page.getByRole('button', { name: ':sample_custom: 3', exact: true });
	await expect(reaction.locator('[data-loading-image]')).toHaveAttribute('data-loading-image', 'error');
	await expect(reaction.getByRole('img', { name: ':sample_custom:: 이미지를 불러올 수 없어요' })).toHaveText(':sample_custom:');
	await expect(reaction.locator('[data-slot="skeleton"]')).toHaveCount(0);
	const countBeforeClick = mutationRequests;
	await reaction.click();
	expect(mutationRequests).toBe(countBeforeClick);
	await expect(reaction).toHaveAttribute('aria-pressed', 'false');
	await page.mouse.move(0, 0);
	await reaction.hover();
	const tooltip = page.locator('[data-slot="tooltip-content"]').filter({ hasText: '박예시님이 눌렀어요' });
	await expect(tooltip.getByRole('img', { name: ':sample_custom:: 이미지를 불러올 수 없어요' })).toHaveText(':sample_custom:');
	if (process.env.LOADING_EVIDENCE_DIRECTORY) await page.screenshot({ path: `${process.env.LOADING_EVIDENCE_DIRECTORY}/custom-emoji-failed.png`, animations: 'disabled' });
});

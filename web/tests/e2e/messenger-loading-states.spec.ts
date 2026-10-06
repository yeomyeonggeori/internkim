import { expect, test, type Page } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { mockDeviceMessenger, type MockConversation } from './messenger-device-mock';

const reader = { id: 'loading-reader', email: 'reader@example.com' };
const conversations: MockConversation[] = [{ id: 'loading-channel', name: '샘플광장', kind: 'group', myRole: 'member' }];
const message = { id: 'loading-message', sender: { id: 'sample-person', name: '이샘플' }, text: '확인된 대화 내용', sentAt: '2026-10-06T02:00:00Z' };
const isBaseline = process.env.LOADING_EVIDENCE_PHASE === 'before';

function gate() {
	let release = () => {};
	const promise = new Promise<void>(resolve => { release = resolve; });
	return { promise, release };
}

async function capture(page: Page, scene: string) {
	const directory = process.env.LOADING_EVIDENCE_DIRECTORY;
	if (!directory) return;
	await mkdir(directory, { recursive: true });
	await page.screenshot({ path: `${directory}/${scene}-${isBaseline ? 'before' : 'after'}.png`, animations: 'disabled' });
}

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });
test.beforeEach(async ({ page }) => {
	expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
	await page.clock.setFixedTime(new Date('2026-10-06T03:00:00Z'));
	await mockDeviceMessenger(page, reader, conversations, [message]);
	await page.route('**/agent/api/person-pictures', route => route.fulfill({ json: {} }));
	await page.route('**/agent/api/custom-emoji', route => route.fulfill({ json: { emojis: [] } }));
});

for (const width of [1280, 390, 320]) {
	test(`cold conversation list reserves content at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const pending = gate();
		const started = gate();
		await page.route('**/agent/api/channels', async route => {
			started.release();
			await pending.promise;
			await route.fulfill({ json: { conversations } });
		});
		await page.goto('/messenger?channel=loading-channel');
		await started.promise;
		if (width < 640) await page.getByRole('button', { name: '채널 목록 열기' }).click();
		if (!isBaseline) await expect(page.locator('[data-messenger-list-skeleton]:visible').first()).toBeVisible();
		else await expect(page.locator('[data-messenger-list-skeleton]')).toHaveCount(0);
		await capture(page, `messenger-cold-${width}`);
		pending.release();
		await expect(page.locator('[data-sidebar="menu-button"]:visible').filter({ hasText: '샘플광장' }).first()).toBeVisible();
		if (width < 640) await page.keyboard.press('Escape');
		await expect(page.getByText(message.text)).toBeVisible();
		await expect(page.locator('[data-messenger-list-skeleton]')).toHaveCount(0);
		await capture(page, `messenger-loaded-${width}`);
	});

	test(`people picker distinguishes pending and empty at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		await page.goto('/messenger?channel=loading-channel');
		await expect(page.getByText(message.text)).toBeVisible();
		const pending = gate();
		const started = gate();
		await page.route('**/agent/api/people', async route => { started.release(); await pending.promise; await route.fulfill({ json: { people: [] } }); });
		if (width < 640) await page.getByRole('button', { name: '채널 목록 열기' }).click();
		await page.getByRole('button', { name: '새 개인 메시지', exact: true }).click();
		await started.promise;
		const dialog = page.getByRole('dialog');
		if (!isBaseline) {
			await expect(dialog.locator('[data-messenger-list-skeleton]')).toBeVisible();
			await expect(dialog.getByText('대화할 사람이 없습니다')).toHaveCount(0);
		} else await expect(dialog.getByText('대화할 사람이 없습니다')).toBeVisible();
		await capture(page, `messenger-people-${width}`);
		pending.release();
		await expect(dialog.getByText('대화할 사람이 없습니다')).toBeVisible();
		await expect(dialog.locator('[data-messenger-list-skeleton]')).toHaveCount(0);
	});
}

test('people failure is an error and retry returns actual people', async ({ page }) => {
	test.skip(isBaseline, 'New error/retry behavior is asserted against implementation');
	await page.goto('/messenger?channel=loading-channel');
	await expect(page.getByText(message.text)).toBeVisible();
	let fail = true;
	await page.route('**/agent/api/people', route => route.fulfill(fail ? { status: 503, body: 'Directory unavailable' } : { json: { people: [{ id: 'sample-person', name: '박예시' }] } }));
	await page.getByRole('button', { name: '새 개인 메시지', exact: true }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('alert')).toHaveText('Directory unavailable');
	await expect(dialog.getByText('대화할 사람이 없습니다')).toHaveCount(0);
	await expect(dialog.locator('[data-messenger-list-skeleton]')).toHaveCount(0);
	fail = false;
	await dialog.getByRole('button', { name: '다시 시도' }).click();
	await expect(dialog.getByText('박예시')).toBeVisible();
});

test('known conversation survives refresh but scope reset hides it until the new list settles', async ({ page }) => {
	test.skip(isBaseline, 'New scope loading behavior is asserted against implementation');
	await page.goto('/messenger?channel=loading-channel');
	await expect(page.getByText(message.text)).toBeVisible();
	const pending = gate();
	await page.route('**/agent/api/channels', async route => { await pending.promise; await route.fulfill({ json: { conversations: [{ ...conversations[0], id: 'replacement', name: '새 범위 대화' }] } }); });
	await page.reload();
	await expect(page.getByText(message.text)).toBeVisible();
	await expect(page.locator('[data-messenger-list-skeleton]')).toHaveCount(0);
	await page.evaluate(async () => { const moduleURL = '/src/lib/messenger/cache-scope.ts'; const scope = await import(moduleURL); scope.invalidateMessengerCacheScope(true); });
	await expect(page.locator('[data-messenger-list-skeleton]').first()).toBeVisible();
	await expect(page.getByText(message.text)).toHaveCount(0);
	pending.release();
	await expect(page.locator('[data-sidebar="menu-button"]').filter({ hasText: '새 범위 대화' })).toBeVisible();
	await expect(page).toHaveURL(/channel=replacement/);
});

test('failed initial conversation list stops skeletons and retries initial selection', async ({ page }) => {
	test.skip(isBaseline, 'New error/retry behavior is asserted against implementation');
	let fail = true;
	await page.route('**/agent/api/channels', route => route.fulfill(fail ? { status: 503, body: 'Conversations unavailable' } : { json: { conversations } }));
	await page.goto('/messenger?channel=loading-channel');
	await expect(page.getByRole('alert').filter({ hasText: 'Conversations unavailable' }).first()).toBeVisible();
	await expect(page.locator('[data-messenger-list-skeleton], [data-message-skeleton]')).toHaveCount(0);
	fail = false;
	await page.getByRole('button', { name: '다시 시도', exact: true }).last().click();
	await expect(page).toHaveURL(/channel=loading-channel/);
	await expect(page.getByText(message.text)).toBeVisible();
});

for (const status of [401, 403]) {
	test(`a loaded people picker clears its directory after a ${status} refresh refusal`, async ({ page }) => {
		test.skip(isBaseline, 'New explicit picker error is asserted against implementation');
		await page.goto('/messenger?channel=loading-channel');
		await expect(page.getByText(message.text)).toBeVisible();
		let denied = false;
		await page.route('**/agent/api/people', route => route.fulfill(denied
			? { status, body: 'Directory access denied' }
			: { json: { people: [{ id: 'sample-person', name: '박예시' }] } }));
		await page.getByRole('button', { name: '새 개인 메시지', exact: true }).click();
		await expect(page.getByRole('dialog').getByText('박예시')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog')).toHaveCount(0);
		denied = true;
		await page.getByRole('button', { name: '새 개인 메시지', exact: true }).click();
		const dialog = page.getByRole('dialog');
		await expect(dialog.getByRole('alert')).toHaveText('Directory access denied');
		await expect(dialog.getByText('박예시')).toHaveCount(0);
		await expect(dialog.getByText('대화할 사람이 없습니다')).toHaveCount(0);
		await expect(dialog.locator('[data-messenger-list-skeleton]')).toHaveCount(0);
	});
}

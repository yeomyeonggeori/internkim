import { expect, test, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

for (const viewport of [{ width: 320, height: 760 }, { width: 568, height: 320 }, { width: 1280, height: 800 }]) {
	test(`shared toasts preserve position, stack and actions at ${viewport.width}px`, async ({ page }) => {
		await openConversation(page, viewport.width, viewport.height);
		await page.evaluate(async () => {
			const url = performance.getEntriesByType('resource').map(entry => entry.name).find(name => /\/svelte-sonner\.js\?/.test(name));
			if (!url) throw new Error('Loaded Sonner module was not found');
			const { toast } = await import(/* @vite-ignore */ url);
			for (let index = 1; index <= 3; index++) toast(`모바일 검토 알림 ${index}`, { duration: Infinity, action: { label: '확인', onClick: () => { document.body.dataset.toastAction = 'done'; } } });
		});
		const toaster = page.locator('[data-sonner-toaster]');
		await expect(toaster).toHaveAttribute('data-y-position', viewport.width < 640 ? 'top' : 'bottom');
		await expect(page.locator('[data-sonner-toast]')).toHaveCount(3);
		const front = page.locator('[data-sonner-toast][data-front="true"]');
		await expect(front).toBeVisible();
		if (viewport.width < 640) {
			await expect.poll(async () => (await front.boundingBox())?.y ?? -1).toBeGreaterThanOrEqual(59.5);
			const box = await front.boundingBox();
			expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height);
			await expectTouchTarget(front.getByRole('button', { name: '닫기', exact: true }));
			await expectTouchTarget(front.getByRole('button', { name: '확인', exact: true }));
			await page.getByRole('button', { name: '더보기', exact: true }).click();
			const action = front.getByRole('button', { name: '확인', exact: true });
			expect(await action.evaluate(element => { const rect = element.getBoundingClientRect(); return element.contains(document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2)); })).toBe(true);
			const output = process.env.MOBILE_UX_SCREENSHOTS;
			if (output) await page.screenshot({ path: `${output}/toast-${viewport.width}.png` });
			await page.keyboard.press('Escape');
			await expect(page.getByRole('dialog', { name: '더보기' })).toBeHidden();
			if (viewport.width === 320) {
				await page.evaluate(() => {
					const viewport = window.visualViewport!;
					Object.defineProperties(viewport, { height: { configurable: true, get: () => 440 }, offsetTop: { configurable: true, get: () => 24 } });
					viewport.dispatchEvent(new Event('resize'));
				});
				await expect.poll(async () => (await front.boundingBox())?.y ?? -1).toBeGreaterThanOrEqual(83.5);
				expect((await front.boundingBox())!.y + (await front.boundingBox())!.height).toBeLessThanOrEqual(464);
				if (output) await page.screenshot({ path: `${output}/toast-keyboard-320.png` });
			}
			await front.getByRole('button', { name: '닫기', exact: true }).click();
			await expect(page.locator('[data-sonner-toast]')).toHaveCount(2);
		}
		await page.locator('[data-sonner-toast][data-front="true"]').getByRole('button', { name: '확인', exact: true }).click();
		await expect(page.locator('body')).toHaveAttribute('data-toast-action', 'done');
		await expectNoHorizontalOverflow(page);
	});
}

const channelID = 'mobile-ux-channel';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };

async function openConversation(page: Page, width: number, height = 760) {
	await page.setViewportSize({ width, height });
	await mockDeviceMessenger(page, reader, [{
		id: channelID,
		name: '모바일 화면에서 긴 채널 이름도 안전하게 표시하는 대화',
		kind: 'group',
		myRole: 'member',
		members: [{ externalID: author.id, name: author.name, role: 'member' }]
	}], [{
		id: 'mobile-ux-message', sender: author,
		text: '사무실 문 앞에 퀵 하나 왔습니다. 챙겨주세요.',
		sentAt: '2026-09-28T01:00:00Z'
	}]);
	await page.goto(`/messenger?channel=${channelID}`);
	await expect(page.getByRole('combobox', { name: '메시지를 입력하세요' })).toBeVisible();
}

async function expectTouchTarget(control: Locator) {
	await expect.poll(async () => (await control.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(44);
	await expect.poll(async () => (await control.boundingBox())?.width ?? 0).toBeGreaterThanOrEqual(44);
}

async function expectNoHorizontalOverflow(page: Page) {
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
		page.viewportSize()?.width ?? 0
	);
}

for (const width of [320, 360, 390]) {
	test(`mobile attachment and selected text formatting fit ${width}px`, async ({ page }) => {
		await openConversation(page, width);
		await expect(page.locator('.internkim-app-header')).toBeHidden();
		const composer = page.getByRole('combobox', { name: '메시지를 입력하세요' });
		const attachment = page.getByRole('button', { name: '파일 첨부' });
		await expectTouchTarget(attachment);
		expect(await attachment.evaluate((element) => getComputedStyle(element).borderWidth)).toBe('0px');
		expect(await attachment.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
		expect(await page.locator('.composer-input').evaluate((element) => parseFloat(getComputedStyle(element).borderRadius))).toBeGreaterThanOrEqual(24);
		await expect(page.getByRole('button', { name: '이모지 넣기' })).toBeHidden();
		await expect(page.getByRole('button', { name: '서식 표시' })).toBeHidden();
		await expectTouchTarget(page.getByRole('button', { name: '채널 목록 열기' }));
		await expectTouchTarget(page.getByRole('button', { name: '채널 정보' }));
		await expectTouchTarget(page.getByRole('button', { name: '보내기', exact: true }));
		expect((await composer.boundingBox())?.height).toBeLessThanOrEqual(48);
		await expectNoHorizontalOverflow(page);
		const directory = process.env.MOBILE_UX_SCREENSHOTS;
		if (directory) await page.screenshot({ path: `${directory}/after-${width}.png` });

		const chooser = page.waitForEvent('filechooser');
		await attachment.click();
		await (await chooser).setFiles({ name: '검토.txt', mimeType: 'text/plain', buffer: Buffer.from('검토') });
		await expect(page.getByText('검토.txt', { exact: true })).toBeVisible();
		await page.getByRole('button', { name: '첨부 제거' }).click();
		await composer.fill('앞😀뒤 한글 문장');
		await composer.focus();
		await composer.evaluate((element: HTMLTextAreaElement) => element.setSelectionRange(5, 7));
		const formatting = page.getByRole('dialog', { name: '선택한 텍스트 서식' });
		await expect(formatting).toBeVisible();
		await expectTouchTarget(page.getByRole('button', { name: '굵게', exact: true }));
		await expectNoHorizontalOverflow(page);
		await page.mouse.move(0, 0);
		await expect(page.getByRole('tooltip')).toBeHidden();
		if (directory) await page.screenshot({ path: `${directory}/after-selection-${width}.png` });
		await page.getByRole('button', { name: '굵게', exact: true }).click();
		await expect(composer).toHaveValue('앞😀뒤 **한글** 문장');
		await expect(composer).toBeFocused();
		await expect(formatting).toBeVisible();
		await page.getByRole('button', { name: '서식 지우기', exact: true }).click();
		await expect(composer).toHaveValue('앞😀뒤 한글 문장');
		await composer.evaluate((element: HTMLTextAreaElement) => element.setSelectionRange(1, 3));
		await page.getByRole('button', { name: '기울임', exact: true }).click();
		await expect(composer).toHaveValue('앞*😀*뒤 한글 문장');
		await composer.evaluate((element: HTMLTextAreaElement) => element.setSelectionRange(0, 0));
		await expect(formatting).toBeHidden();
		await composer.fill('여러 줄의 메시지를 입력합니다.\n'.repeat(20));
		expect((await composer.boundingBox())?.height).toBeLessThanOrEqual(160);
		await expectTouchTarget(page.getByRole('button', { name: '보내기', exact: true }));
	});
}

test('mobile More retains global actions and returns focus', async ({ page }) => {
	await openConversation(page, 390);
	const more = page.getByRole('button', { name: '더보기', exact: true });
	await more.click();
	const sheet = page.getByRole('dialog', { name: '더보기' });
	await expect(sheet.getByRole('button', { name: '새로고침' })).toBeVisible();
	await expect(sheet.getByRole('button', { name: '언어 변경' })).toBeVisible();
	await expect(sheet.getByRole('button', { name: 'Toggle theme' })).toBeVisible();
	await sheet.getByRole('button', { name: 'Toggle theme' }).click();
	await expect(page.locator('html')).toHaveClass(/dark/);
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();
	await expect(more).toBeFocused();
	if (process.env.MOBILE_UX_SCREENSHOTS) {
		await page.screenshot({ path: `${process.env.MOBILE_UX_SCREENSHOTS}/after-dark-390.png` });
	}
});

test('landscape keeps navigation below the composer', async ({ page }) => {
	await openConversation(page, 568, 320);
	const composer = await page.locator('.channel-composer').boundingBox();
	const navigation = await page.locator('.internkim-app-mobile-navigation').boundingBox();
	expect(composer && navigation && composer.y + composer.height <= navigation.y).toBe(true);
	await expectNoHorizontalOverflow(page);
	if (process.env.MOBILE_UX_SCREENSHOTS) {
		await page.screenshot({ path: `${process.env.MOBILE_UX_SCREENSHOTS}/after-landscape.png` });
	}
});

test('mobile search opens from More and follows the keyboard viewport', async ({ page }) => {
	await openConversation(page, 320);
	await page.evaluate(() => {
		const viewport = window.visualViewport!;
		Object.defineProperty(viewport, 'height', { configurable: true, value: 440 });
		Object.defineProperty(viewport, 'offsetTop', { configurable: true, value: 20 });
		viewport.dispatchEvent(new Event('resize'));
	});
	await page.getByRole('button', { name: '더보기', exact: true }).click();
	await page.getByRole('dialog', { name: '더보기' }).getByRole('button', { name: '검색', exact: true }).click();
	const search = page.getByRole('dialog', { name: '검색', exact: true });
	await expect(search).toBeVisible();
	await expect(search.getByRole('combobox')).toBeFocused();
	const bounds = await search.boundingBox();
	expect(bounds!.x).toBeGreaterThanOrEqual(0);
	expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(320);
	expect(bounds!.y).toBeGreaterThanOrEqual(20);
	expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(460);
	await page.keyboard.press('Escape');
	await expect(search).toBeHidden();
});

test('desktop retains its global header and direct composer tools', async ({ page }) => {
	await openConversation(page, 1280, 800);
	await expect(page.locator('.internkim-app-header')).toBeVisible();
	await expect(page.locator('.internkim-app-mobile-navigation')).toBeHidden();
	await expect(page.getByRole('button', { name: '파일 첨부' })).toBeVisible();
	await expect(page.getByRole('button', { name: '서식 표시' })).toBeVisible();
	await expectNoHorizontalOverflow(page);
	if (process.env.MOBILE_UX_SCREENSHOTS) {
		await page.screenshot({ path: `${process.env.MOBILE_UX_SCREENSHOTS}/after-desktop.png` });
	}
});

test('software keyboard viewport and bottom safe area keep the composer reachable', async ({ page }) => {
	await openConversation(page, 390);
	await page.getByRole('combobox', { name: '메시지를 입력하세요' }).focus();
	await page.evaluate(() => {
		const viewport = window.visualViewport;
		if (!viewport) throw new Error('This browser must support visualViewport');
		document.documentElement.style.setProperty('--app-mobile-nav-bottom', '34px');
		Object.defineProperty(viewport, 'height', { configurable: true, value: 440 });
		Object.defineProperty(viewport, 'offsetTop', { configurable: true, value: 0 });
		viewport.dispatchEvent(new Event('resize'));
	});
	const navigation = page.locator('.internkim-app-mobile-navigation');
	const composer = page.locator('.channel-composer');
	await expect.poll(async () => {
		const navigationBounds = await navigation.boundingBox();
		const composerBounds = await composer.boundingBox();
		return !!navigationBounds && !!composerBounds &&
			navigationBounds.y + navigationBounds.height <= 440 &&
			composerBounds.y + composerBounds.height <= navigationBounds.y;
	}).toBe(true);
	await expectTouchTarget(page.getByRole('button', { name: '파일 첨부' }));
	await page.evaluate(() => {
		const viewport = window.visualViewport;
		if (!viewport) throw new Error('This browser must support visualViewport');
		Object.defineProperty(viewport, 'scale', { configurable: true, value: 2 });
		viewport.dispatchEvent(new Event('resize'));
	});
	expect(await page.evaluate(() => document.documentElement.style.getPropertyValue('--app-viewport-height'))).toBe('');
});

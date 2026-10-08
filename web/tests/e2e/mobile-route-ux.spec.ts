import { expect, test, type Page } from '@playwright/test';
import { expectTouchTarget } from './touch-target';
import { mockBuzzDisabled } from './buzz-test-routes';
import { routeCalendarShellAPI } from './calendar-route-shell-test-utils';

async function screenshot(page: Page, name: string) {
	const output = process.env.MOBILE_UX_SCREENSHOTS;
	if (output) await page.screenshot({ path: `${output}/${name}.png` });
}

async function fits(page: Page) {
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width);
}

test.beforeEach(async ({ page }) => {
	page.setDefaultTimeout(10_000);
	await mockBuzzDisabled(page);
	await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/persona/api/user', route => route.fulfill({ json: { schemaVersion: 1, callMe: '이샘플', morningBriefing: { enabled: true, time: '09:00' } } }));
});

for (const width of [320, 390, 1280]) {
	test(`calendar toolbar keeps its controls on one compact mobile row at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 844 });
		await routeCalendarShellAPI(page);
		await page.route('**/api/v1/tools/event_list/invoke', route => route.fulfill({ json: { result: { events: [] } } }));
		await page.route('**/api/v1/tools/company_settings_get/invoke', route => route.fulfill({ json: { result: { timeZone: 'Asia/Seoul' } } }));
		await page.goto('/calendar/');
		await expect(page.locator('.calendar-stage')).toBeVisible();
		const toolbar = page.locator('.calendar-toolbar');
		await fits(page);
		if (width < 640) {
			const title = await toolbar.locator('.calendar-toolbar-title').boundingBox();
			const more = toolbar.getByRole('button', { name: '월 일정 도구', exact: true });
			const moreBox = await more.boundingBox();
			const today = await toolbar.getByRole('button', { name: '오늘', exact: true }).boundingBox();
			expect(title && moreBox && today).toBeTruthy();
			expect(Math.abs(title!.y + title!.height / 2 - (moreBox!.y + moreBox!.height / 2))).toBeLessThan(1);
			expect(Math.abs(today!.y - moreBox!.y)).toBeLessThan(1);
			expect(moreBox!.height).toBe(36);
			expect(today!.height).toBe(36);
			await expectTouchTarget(more);
			await expectTouchTarget(toolbar.getByRole('button', { name: '오늘', exact: true }));
			await expect(toolbar.getByRole('tablist')).toBeHidden();
			await expect(toolbar.getByRole('combobox', { name: '참여자 선택', exact: true })).toBeHidden();
			await screenshot(page, `calendar-compact-${width}`);
			await more.click();
			await expect(page.getByRole('tab', { name: '주', exact: true })).toBeVisible();
			const participant = page.getByRole('combobox', { name: '참여자 선택', exact: true });
			await expect(participant).toBeVisible();
			await screenshot(page, `calendar-tools-${width}`);
			await participant.click();
			await expect(page.getByRole('combobox').last()).toBeVisible();
			await page.keyboard.press('Escape');
			await page.getByRole('button', { name: '설정', exact: true }).click();
			await expect(page.getByRole('dialog')).toBeVisible();
			await fits(page);
		} else {
			await expect(toolbar.getByRole('button', { name: '월 일정 도구', exact: true })).toBeHidden();
			await expect(toolbar.getByRole('combobox', { name: '참여자 선택', exact: true })).toBeVisible();
			await screenshot(page, `calendar-compact-${width}`);
		}
	});
}

for (const width of [320, 390]) {
	test(`files retain mobile root selection and shared preview after reader-role integration at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 844 });
		await page.goto('/files/');
		await expect(page.getByRole('tab', { name: '데이터룸', exact: true })).toBeVisible();
		const item = page.getByRole('button', { name: /주간-회고.md/ });
		await expect(item).toBeVisible();
		await expectTouchTarget(item);
		await page.getByRole('button', { name: '위치', exact: true }).click();
		await page.getByRole('option', { name: '공개', exact: true }).click();
		await page.getByRole('button', { name: /공지사항.md/ }).click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await fits(page);
	});
}

for (const viewport of [{ width: 320, height: 760 }, { width: 360, height: 760 }, { width: 390, height: 844 }, { width: 568, height: 320 }, { width: 1280, height: 800 }]) {
	test(`workspace surfaces preserve mobile controls at ${viewport.width}px`, async ({ page }) => {
		test.setTimeout(150_000);
		await page.setViewportSize(viewport);
		const mobile = viewport.width < 640;
		await page.goto('/files/');
		await expect(page.getByRole('button', { name: /주간-회고.md/ })).toBeVisible();
		await fits(page);
		await screenshot(page, `files-${viewport.width}`);
		if (mobile) {
			await page.getByRole('button', { name: '위치', exact: true }).click();
			await screenshot(page, `files-roots-${viewport.width}`);
			await page.getByRole('option', { name: '공개', exact: true }).click();
			await expect(page.getByRole('button', { name: /공지사항.md/ })).toBeVisible();
			await page.getByRole('button', { name: /공지사항.md/ }).click();
			await expect(page.getByRole('dialog')).toBeVisible();
			await fits(page);
			await screenshot(page, `files-detail-${viewport.width}`);
			await page.keyboard.press('Escape');
			await expect(page.getByRole('dialog')).toBeHidden();
		}
		await page.goto('/mail/');
		await expect(page.getByText('메일 캐시 동작 확인', { exact: true }).first()).toBeVisible();
		await fits(page);
		await page.getByText('메일 캐시 동작 확인', { exact: true }).first().click();
		await expect(page.getByRole('button', { name: '답장', exact: true })).toBeVisible();
		await fits(page);
		await screenshot(page, `mail-${viewport.width}`);
		if (mobile) {
			await page.getByRole('button', { name: '더 보기', exact: true }).click();
			await expect(page.getByRole('menuitem', { name: '전달', exact: true })).toBeVisible();
			await page.keyboard.press('Escape');
		} else {
			await page.getByRole('button', { name: '더 보기', exact: true }).focus();
			await page.keyboard.press('ArrowDown');
			await expect(page.getByRole('menuitem', { name: '읽지 않음으로 표시', exact: true })).toBeFocused();
			await page.keyboard.press('Escape');
		}
		await page.goto('/crm/');
		await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
		if (mobile) {
			await page.getByRole('button', { name: 'CRM 현황', exact: true }).click();
			await expect(page.locator('[data-crm-metrics]')).toContainText('열린 거래 금액');
			await page.getByRole('button', { name: 'CRM 현황', exact: true }).click();
		}
		for (const tabName of ['관계처', '연락처', '거래', '활동', '보고']) {
			await page.getByRole('tab', { name: tabName, exact: true }).click();
			const panel = page.getByRole('tabpanel', { name: tabName });
			await expect(panel).toBeVisible();
			await fits(page);
			await screenshot(page, `crm-${tabName}-${viewport.width}`);
			if (mobile && (tabName === '연락처' || tabName === '거래')) {
				await panel.getByRole('button', { name: '정렬', exact: true }).click();
				await expect(panel.getByRole('button', { name: /관계처/ }).first()).toBeVisible();
				await panel.getByRole('button', { name: /관계처/ }).first().click();
			}
		}
		await page.goto('/memory/schedules/');
		await expect(page.locator('[data-slot="skeleton"]')).toHaveCount(0);
		await expect(page.getByRole('button', { name: '수정', exact: true }).first()).toBeVisible();
		await fits(page);
		await screenshot(page, `schedules-${viewport.width}`);
		await page.getByRole('button', { name: '수정', exact: true }).first().click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await fits(page);
		await screenshot(page, `schedule-edit-${viewport.width}`);
		await page.keyboard.press('Escape');
		await page.goto('/runs/');
		await expect(page.locator(viewport.width < 768 ? '[data-task-run-mobile-list] > *' : 'tbody tr').first()).toBeVisible();
		await fits(page);
		await screenshot(page, `runs-${viewport.width}`);
		await page.goto('/runs/dev-task-run-001');
		await expect(page.locator('main')).toBeVisible();
		await fits(page);
		await screenshot(page, `run-detail-${viewport.width}`);
		await page.goto('/settings/');
		await expect(page.getByRole('tab').first()).toBeVisible();
		await fits(page);
		await screenshot(page, `settings-${viewport.width}`);
		await page.goto('/settings/setup/');
		await expect(page.locator('main')).toBeVisible();
		await fits(page);
		await screenshot(page, `setup-${viewport.width}`);
		await routeCalendarShellAPI(page);
		await page.goto('/calendar/');
		await expect(page.locator('.calendar-stage')).toBeVisible();
		await fits(page);
		await screenshot(page, `calendar-${viewport.width}`);
		await page.goto('/calendar/embed/');
		await expect(page.locator('.calendar-stage')).toBeVisible();
		await fits(page);
		await screenshot(page, `calendar-embed-${viewport.width}`);
	});
}

test('the app keeps clear of the status bar and home indicator and its tabs answer down to the screen edge', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await routeCalendarShellAPI(page);
	await page.route('**/api/v1/tools/event_list/invoke', route => route.fulfill({ json: { result: { events: [] } } }));
	await page.route('**/api/v1/tools/company_settings_get/invoke', route => route.fulfill({ json: { result: { timeZone: 'Asia/Seoul' } } }));
	const session = await page.context().newCDPSession(page);
	await session.send('Emulation.setSafeAreaInsetsOverride', { insets: { top: 47, bottom: 34, left: 0, right: 0 } });
	await page.goto('/calendar/');
	await expect(page.locator('meta[name="viewport"]')).toHaveAttribute('content', /viewport-fit=cover/);
	const navigation = page.locator('.internkim-app-mobile-navigation');
	await expect(navigation).toBeVisible();
	const headerTop = await page.locator('.internkim-app-header').evaluate(header => header.getBoundingClientRect().top);
	expect(headerTop).toBeGreaterThanOrEqual(47);
	const lastLabelBottom = await navigation.locator('a, button').last().locator('span').evaluate(label => label.getBoundingClientRect().bottom);
	expect(844 - lastLabelBottom).toBeGreaterThanOrEqual(34);
	const tabsUnderTheScreenEdge = await navigation.locator('a, button').evaluateAll(items =>
		items.map(item => {
			const bounds = item.getBoundingClientRect();
			return document.elementFromPoint(bounds.left + bounds.width / 2, 843)?.closest('a, button') === item;
		})
	);
	expect(tabsUnderTheScreenEdge).toEqual([true, true, true, true, true]);
});

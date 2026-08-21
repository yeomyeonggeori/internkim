import { expect, test } from '@playwright/test';
import { flowDashboardTaskID, openFlowBoard, taskCard, visibleBoundingBox } from './flow-task-helpers';

test.describe('flow task quick add', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('opens AI quick add as a bottom floating conversation panel', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);

		const closedLauncher = page.getByRole('button', { name: 'AI로 업무 추가', exact: true });
		await expect(closedLauncher).toBeVisible();
		await closedLauncher.click();

		const openLauncher = page.getByRole('button', { name: 'AI 업무 추가 닫기', exact: true });
		await expect(openLauncher).toBeVisible();
		const panel = page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true });
		const quickAddInput = page.getByPlaceholder('예: 10분 회의');
		const viewportSize = page.viewportSize();
		if (!viewportSize) throw new Error('viewport size was not available');

		await expect(panel).toHaveCSS('transition-duration', '0.4s');
		await expect(quickAddInput).toBeFocused();
		await expect(panel).toHaveCSS('opacity', '1');
		const panelBox = await visibleBoundingBox(panel);
		expect(panelBox.width).toBeGreaterThanOrEqual(390);
		expect(panelBox.width).toBeLessThanOrEqual(420);
		expect(panelBox.height).toBeGreaterThanOrEqual(490);
		expect(panelBox.height).toBeLessThanOrEqual(510);
		expect(panelBox.height).toBeGreaterThan(panelBox.width * 1.2);
		expect(panelBox.x + panelBox.width).toBeGreaterThan(viewportSize.width - 40);

		await quickAddInput.fill('새 업무');
		await page.keyboard.press('Tab');
		await expect(panel.getByRole('button', { name: 'AI로 업무 추가', exact: true })).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(openLauncher).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(quickAddInput).toBeFocused();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true })).toHaveCount(0);
		await expect(closedLauncher).toBeFocused();
		await expect(closedLauncher).toBeVisible();
	});

	test('closes AI quick add after creation and restores launcher focus', async ({ page }) => {
		await page.route('**/flow/api/tasks/quick', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ status: 'created', reason: '' })
			});
		});
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);

		const launcher = page.getByRole('button', { name: 'AI로 업무 추가', exact: true });
		await launcher.click();
		const panel = page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true });
		await panel.getByPlaceholder('예: 10분 회의').fill('AI 성공 닫힘 확인');
		await panel.getByRole('button', { name: 'AI로 업무 추가', exact: true }).click();

		await expect(panel).toHaveCount(0);
		await expect(launcher).toBeFocused();
	});

	test('keeps AI quick add open when the quick task is a duplicate', async ({ page }) => {
		await page.route('**/flow/api/tasks/quick', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ status: 'skipped_duplicate', reason: 'duplicate' })
			});
		});
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);

		await page.getByRole('button', { name: 'AI로 업무 추가', exact: true }).click();
		const panel = page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true });
		await panel.getByPlaceholder('예: 10분 회의').fill('중복 유지 확인');
		await panel.getByRole('button', { name: 'AI로 업무 추가', exact: true }).click();

		await expect(panel).toBeVisible();
		await expect(panel.getByText('이미 있는 업무로 보여 추가하지 않았습니다.', { exact: true })).toBeVisible();
		await expect(panel.getByRole('button', { name: '그래도 추가', exact: true })).toBeVisible();
	});

	test('hides AI quick add while the task sidebar is open', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);

		const launcher = page.getByRole('button', { name: 'AI로 업무 추가', exact: true });
		await expect(launcher).toBeVisible();

		await taskCard(page, flowDashboardTaskID).click();
		await expect(page.getByRole('dialog', { name: '업무 상세', exact: true })).toBeVisible();
		await expect(launcher).toHaveCount(0);

		await page.getByRole('dialog').getByRole('button', { name: '업무 수정', exact: true }).click();
		await expect(page.getByRole('dialog', { name: '업무 수정', exact: true })).toBeVisible();
		await expect(launcher).toHaveCount(0);

		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog', { name: '업무 수정', exact: true })).toHaveCount(0);
		await expect(launcher).toBeVisible();
	});

	test('keeps AI quick add above mobile navigation and hides it behind mobile sheets', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await openFlowBoard(page);

		const launcher = page.getByRole('button', { name: 'AI로 업무 추가', exact: true });
		const bottomNavigation = page.locator('nav[aria-label]').filter({ has: page.getByRole('button', { name: '더보기', exact: true }) });
		const launcherBox = await visibleBoundingBox(launcher);
		const bottomNavigationBox = await visibleBoundingBox(bottomNavigation);

		expect(bottomNavigationBox.y - (launcherBox.y + launcherBox.height)).toBeGreaterThanOrEqual(12);
		expect(Math.abs((launcherBox.x + launcherBox.width) - (bottomNavigationBox.x + bottomNavigationBox.width))).toBeLessThanOrEqual(1);

		await bottomNavigation.getByRole('button', { name: '더보기', exact: true }).click();
		const moreSheet = page.locator('[data-slot="sheet-content"][data-state="open"]');
		await expect(moreSheet).toBeVisible();
		await expect(moreSheet.getByRole('button', { name: '닫기', exact: true })).toBeVisible();
		await expect(moreSheet.getByRole('link', { name: '기억', exact: true })).toBeVisible();
		await expect(moreSheet.getByRole('link', { name: '파일', exact: true })).toBeVisible();
		await expect(moreSheet.getByRole('link', { name: '작업 기록', exact: true })).toBeVisible();
		await expect(launcher).toHaveCSS('opacity', '0');

		await page.keyboard.press('Escape');
		await expect(launcher).toHaveCSS('opacity', '1');
	});
});

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
		const closedLauncherBox = await visibleBoundingBox(closedLauncher);
		await closedLauncher.click();

		const openLauncher = page.getByRole('button', { name: 'AI 업무 추가 닫기', exact: true });
		await expect(openLauncher).toBeVisible();
		await expect(openLauncher.locator('[data-flow-quick-add-closed-content]')).toBeAttached();
		await expect(openLauncher.locator('[data-flow-quick-add-open-content]')).toBeAttached();
		await expect.poll(async () => {
			const box = await openLauncher.boundingBox();
			return box?.width ?? 0;
		}).toBeLessThan(closedLauncherBox.width * 0.5);
		const openLauncherBox = await visibleBoundingBox(openLauncher);
		const panel = page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true });
		const panelBox = await visibleBoundingBox(panel);
		const quickAddInput = page.getByPlaceholder('예: 10분 회의');
		const viewportSize = page.viewportSize();
		if (!viewportSize) throw new Error('viewport size was not available');

		await expect(panel).toHaveCSS('transition-duration', '0.4s');
		await expect(quickAddInput).toBeFocused();
		expect(openLauncherBox.width).toBeLessThan(closedLauncherBox.width * 0.5);
		expect(Math.abs(openLauncherBox.x + openLauncherBox.width - (closedLauncherBox.x + closedLauncherBox.width))).toBeLessThan(4);
		expect(Math.abs(openLauncherBox.y + openLauncherBox.height - (closedLauncherBox.y + closedLauncherBox.height))).toBeLessThan(4);
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

	test('hides AI quick add while the task sidebar is open', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);

		const launcher = page.getByRole('button', { name: 'AI로 업무 추가', exact: true });
		await expect(launcher).toBeVisible();

		await taskCard(page, flowDashboardTaskID).click();
		await expect(page.getByRole('dialog', { name: '업무 수정', exact: true })).toBeVisible();
		await expect(launcher).toHaveCount(0);

		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog', { name: '업무 수정', exact: true })).toHaveCount(0);
		await expect(launcher).toBeVisible();
	});
});

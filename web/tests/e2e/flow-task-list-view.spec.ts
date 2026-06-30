import { expect, test } from '@playwright/test';
import { openFlowBoard, visibleBoundingBox } from './flow-task-helpers';

test.describe('flow task list view', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('updates a task status from the list status cell', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);
		await page.getByRole('tab', { name: '목록', exact: true }).click();

		const taskRow = page.getByRole('row', { name: /메일 분류 규칙 정리.*2026-06-04/ });
		await taskRow.getByRole('button', { name: '예정', exact: true }).click();
		await page.getByRole('option', { name: '진행', exact: true }).click();

		await expect(taskRow.getByRole('button', { name: '진행', exact: true })).toBeVisible();
	});

	test('keeps mobile list pagination above AI quick add at the bottom', async ({ page }) => {
		await page.setViewportSize({ width: 430, height: 932 });
		await openFlowBoard(page);
		await page.getByRole('tab', { name: '목록', exact: true }).click();

		await page.locator('[data-flow-task-results]').evaluate((element) => {
			let scrollParent = element.parentElement;
			while (scrollParent && !['auto', 'scroll', 'overlay'].includes(getComputedStyle(scrollParent).overflowY)) {
				scrollParent = scrollParent.parentElement;
			}
			if (!scrollParent) throw new Error('flow task scroll parent was not found');
			scrollParent.scrollTop = scrollParent.scrollHeight;
			scrollParent.dispatchEvent(new Event('scroll'));
		});

		const launcherBox = await visibleBoundingBox(page.getByRole('button', { name: 'AI로 업무 추가', exact: true }));
		const nextButtonBox = await visibleBoundingBox(page.getByRole('button', { name: '다음', exact: true }));

		expect(launcherBox.y - (nextButtonBox.y + nextButtonBox.height)).toBeGreaterThanOrEqual(12);
	});
});

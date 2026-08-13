import { expect, test, type Page } from '@playwright/test';
import { isUnknownRecord, openFlowBoard, requestedTaskID, visibleBoundingBox } from './flow-task-helpers';

const normalStatusLabels = ['예정', '진행', '완료', '일시정지', '중단'];
const requestedStatusLabels = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];

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

	test('offers five statuses for normal work and seven for requested work after status changes', async ({ page }) => {
		await addRequesterProvenance(page);
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);
		await page.getByRole('tab', { name: '목록', exact: true }).click();

		const normalTaskRow = page.getByRole('row', { name: /메일 분류 규칙 정리.*2026-06-04/ });
		await normalTaskRow.getByRole('button', { name: '예정', exact: true }).click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(normalStatusLabels);
		await page.keyboard.press('Escape');

		const requestedTaskRow = page.getByRole('row', { name: /근태 위치 정책 초안 작성.*2026-06-03/ });
		await requestedTaskRow.getByRole('button', { name: '요청', exact: true }).click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
		await page.getByRole('option', { name: '진행', exact: true }).click();
		await requestedTaskRow.getByRole('button', { name: '진행', exact: true }).click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
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

async function addRequesterProvenance(page: Page): Promise<void> {
	await page.route('**/flow/api/state**', async (route) => {
		const response = await route.fetch();
		const state: unknown = await response.json();
		if (!isUnknownRecord(state)) throw new Error('flow state response was not an object');
		const tasks = Array.isArray(state.tasks)
			? state.tasks.map((task) => isUnknownRecord(task) && task.id === requestedTaskID
				? { ...task, requesterID: '', requesterName: '박예시', wasRequested: true }
				: task)
			: [];
		await route.fulfill({ response, json: { ...state, source: 'supabase', tasks } });
	});
}

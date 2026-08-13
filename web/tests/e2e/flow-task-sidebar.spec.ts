import { expect, test, type Page } from '@playwright/test';
import {
	flowDashboardTaskID,
	isUnknownRecord,
	marketScanTaskID,
	openFlowBoard,
	requestedTaskID,
	taskCard,
	useMemberFlowSession
} from './flow-task-helpers';

const normalStatusLabels = ['예정', '진행', '완료', '일시정지', '중단'];
const requestedStatusLabels = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];

test.describe('flow task sidebar', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('explains why a non-participant task editor is read-only', async ({ page }) => {
		await useMemberFlowSession(page, 'designer@example.com', '이영희');
		await openFlowBoard(page);

		await taskCard(page, marketScanTaskID).click();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toBeVisible();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '업무 삭제', exact: true })).toHaveCount(0);
	});

	test('shows no owner or requester for normal work and offers five statuses', async ({ page }) => {
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();
		const sidebar = page.getByRole('dialog');
		await expect(sidebar.getByText('담당자', { exact: true })).toHaveCount(0);
		await expect(sidebar.getByText('요청자', { exact: true })).toHaveCount(0);
		await sidebar.getByRole('button', { name: '업무 수정', exact: true }).click();
		await sidebar.getByRole('button', { name: '상태', exact: true }).click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(normalStatusLabels);
	});

	test('shows requester as read-only and keeps seven statuses after a status change', async ({ page }) => {
		await addRequesterProvenance(page);
		await openFlowBoard(page);

		await taskCard(page, requestedTaskID).click();
		const sidebar = page.getByRole('dialog');
		await expect(sidebar.getByText('요청자', { exact: true })).toBeVisible();
		await expect(sidebar.getByText('이영희', { exact: true })).toBeVisible();
		await expect(sidebar.getByRole('button', { name: '요청자', exact: true })).toHaveCount(0);
		await sidebar.getByRole('button', { name: '업무 수정', exact: true }).click();

		const statusTrigger = sidebar.getByRole('button', { name: '상태', exact: true });
		await statusTrigger.click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
		await page.getByRole('option', { name: '진행', exact: true }).click();
		await statusTrigger.click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
	});

	test('lets a sole participant manage task assignment', async ({ page }) => {
		await useMemberFlowSession(page, 'researcher@example.com', '임수아');
		await openFlowBoard(page);

		await taskCard(page, marketScanTaskID).click();
		await page.getByRole('dialog').getByRole('button', { name: '업무 수정', exact: true }).click();
		await expect(page.getByPlaceholder('이름을 입력해 추가')).toBeEnabled();
	});

	test('does not give assignment authority to one participant in a multi-participant task', async ({ page }) => {
		await useMemberFlowSession(page, 'engineer@example.com', '박민준');
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();
		await page.getByRole('dialog').getByRole('button', { name: '업무 수정', exact: true }).click();
		await expect(page.getByPlaceholder('업무 내용')).toBeEnabled();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toBeVisible();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toHaveCount(0);
		await expect(page.getByPlaceholder('이름을 입력해 추가')).toBeDisabled();
		await expect(page.getByRole('button', { name: /제거$/ })).toHaveCount(0);
	});

	test('keeps task editor dropdown options interactive above the sheet', async ({ page }) => {
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();
		await page.getByRole('dialog').getByRole('button', { name: '업무 수정', exact: true }).click();

		for (const selection of [
			{ trigger: '상태', option: '완료', expected: '완료' },
			{ trigger: '사업', option: '김인턴', expected: '김인턴' },
			{ trigger: '종류', option: '운동', expected: '운동' },
			{ trigger: '크기', option: 'L · 5km · 16h', expected: 'L · 5km · 16h' }
		]) {
			const trigger = page.getByRole('button', { name: selection.trigger, exact: true });
			await trigger.click();
			await page.getByRole('option', { name: selection.option, exact: true }).click();
			await expect(trigger).toContainText(selection.expected);
		}
	});
});

async function addRequesterProvenance(page: Page): Promise<void> {
	await page.route('**/flow/api/state**', async (route) => {
		const response = await route.fetch();
		const state: unknown = await response.json();
		if (!isUnknownRecord(state)) throw new Error('flow state response was not an object');
		const tasks = Array.isArray(state.tasks)
			? state.tasks.map((task) => isUnknownRecord(task) && task.id === requestedTaskID
				? { ...task, requesterID: 'designer', requesterName: '이영희' }
				: task)
			: [];
		await route.fulfill({ response, json: { ...state, tasks } });
	});
}

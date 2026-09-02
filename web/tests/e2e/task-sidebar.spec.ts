import { expect, test, type Page } from '@playwright/test';
import {
	taskDashboardTaskID,
	isUnknownRecord,
	openTaskBoard,
	requestedTaskID,
	taskCard,
	useMemberTaskSession
} from './task-helpers';

const requestedStatusLabels = ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'];

test.describe('flow task sidebar', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/task/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('explains why a non-participant task editor is read-only', async ({ page }) => {
		await useMemberTaskSession(page, 'designer@example.com', '박예시');
		await openTaskBoard(page);

		await taskCard(page, requestedTaskID).click();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toBeVisible();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '업무 삭제', exact: true })).toHaveCount(0);
	});

	test('shows no owner but shows requester for device work and preserves its seven-status contract', async ({ page }) => {
		await openTaskBoard(page);

		await taskCard(page, taskDashboardTaskID).click();
		const sidebar = page.getByRole('dialog');
		await expect(sidebar.getByText('담당자', { exact: true })).toHaveCount(0);
		await expect(sidebar.getByText('요청자', { exact: true })).toBeVisible();
		await sidebar.getByRole('button', { name: '업무 수정', exact: true }).click();
		await sidebar.getByRole('button', { name: '상태', exact: true }).click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
	});

	test('shows distinct participants with the same display name in task detail', async ({ page }) => {
		await addDuplicateParticipantNames(page);
		await openTaskBoard(page);

		await taskCard(page, taskDashboardTaskID).click();
		const participants = page.getByRole('dialog').getByText('이샘플', { exact: true });
		await expect(participants).toHaveCount(2);
		await expect(participants.nth(0)).toBeVisible();
		await expect(participants.nth(1)).toBeVisible();
	});

	test('edits duplicate-named participants by canonical ID and preserves the device owner payload', async ({ page }) => {
		let savedPayload: unknown = null;
		await addDuplicateParticipantNames(page, true);
		await page.route(`**/task/api/tasks/${taskDashboardTaskID}`, async (route) => {
			if (route.request().method() !== 'PUT') return route.continue();
			const document = route.request().postData();
			savedPayload = document ? JSON.parse(document) as unknown : null;
			await route.fulfill({ status: 200 });
		});
		await openTaskBoard(page);

		await taskCard(page, taskDashboardTaskID).click();
		const sidebar = page.getByRole('dialog');
		await sidebar.getByRole('button', { name: '업무 수정', exact: true }).click();
		await sidebar.getByRole('combobox', { name: '참여자', exact: true }).click();
		await expect(page.getByRole('option').filter({ hasText: 'kim@example.com' })).toBeVisible();
		const engineerOption = page.getByRole('option').filter({ hasText: 'engineer@example.com' });
		await expect(engineerOption).toContainText('이샘플');
		await engineerOption.click();
		await page.keyboard.press('Escape');
		await sidebar.getByRole('button', { name: '업무 저장', exact: true }).click();

		expect(isUnknownRecord(savedPayload)).toBe(true);
		if (!isUnknownRecord(savedPayload)) throw new Error('saved flow task payload was not an object');
		expect(savedPayload.ownerID).toBe('kim-intern');
		expect(savedPayload.participantIDs).toEqual(['kim-intern']);
	});

	test('shows requester as read-only and keeps seven statuses after a status change', async ({ page }) => {
		await addRequesterProvenance(page);
		await openTaskBoard(page);

		await taskCard(page, requestedTaskID).click();
		const sidebar = page.getByRole('dialog');
		const requesterLabel = sidebar.getByText('요청자', { exact: true });
		await expect(requesterLabel).toBeVisible();
		await expect(sidebar.getByText('박예시', { exact: true })).toBeVisible();
		await expect(requesterLabel.locator('..').locator('svg')).toBeVisible();
		await expect(requesterLabel.locator('..').locator('input, select, button, [role="combobox"]')).toHaveCount(0);
		await sidebar.getByRole('button', { name: '업무 수정', exact: true }).click();
		await expect(requesterLabel.locator('..').locator('svg')).toBeVisible();
		await expect(requesterLabel.locator('..').locator('input, select, button, [role="combobox"]')).toHaveCount(0);
		const statusTrigger = sidebar.getByRole('button', { name: '상태', exact: true });
		const requesterBox = await requesterLabel.locator('..').boundingBox();
		const statusBox = await statusTrigger.locator('..').boundingBox();
		expect(requesterBox?.y).toBe(statusBox?.y);

		await statusTrigger.click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
		await page.getByRole('option', { name: 'in_progress', exact: true }).click();
		await statusTrigger.click();
		await expect(page.getByRole('listbox').getByRole('option')).toHaveText(requestedStatusLabels);
	});

	test('lets a sole participant manage task assignment', async ({ page }) => {
		await useMemberTaskSession(page, 'planner@example.com', '최견본');
		await openTaskBoard(page);

		await taskCard(page, '26W23-roadmap-review').click();
		const sidebar = page.getByRole('dialog');
		await sidebar.getByRole('button', { name: '업무 수정', exact: true }).click();
		const participantPicker = sidebar.getByRole('combobox', { name: '참여자', exact: true });
		await expect(participantPicker).toBeEnabled();
		await participantPicker.click();
		const selfOption = page.getByRole('option').filter({ hasText: 'planner@example.com' });
		await expect(selfOption).toHaveAttribute('data-checked', 'true');
		await selfOption.click();
		await expect(selfOption).toHaveAttribute('data-checked', 'true');
	});

	test('does not give assignment authority to one participant in a multi-participant task', async ({ page }) => {
		await useMemberTaskSession(page, 'engineer@example.com', '이샘플');
		await openTaskBoard(page);

		await taskCard(page, taskDashboardTaskID).click();
		await page.getByRole('dialog').getByRole('button', { name: '업무 수정', exact: true }).click();
		await expect(page.getByPlaceholder('업무 내용')).toBeEnabled();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toBeVisible();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toHaveCount(0);
		await expect(page.getByRole('dialog').getByRole('combobox', { name: '참여자', exact: true })).toBeDisabled();
		await expect(page.getByRole('button', { name: /제거$/ })).toHaveCount(0);
	});

	test('keeps task editor dropdown options interactive above the sheet', async ({ page }) => {
		await openTaskBoard(page);

		await taskCard(page, taskDashboardTaskID).click();
		await page.getByRole('dialog').getByRole('button', { name: '업무 수정', exact: true }).click();

		for (const selection of [
			{ trigger: '상태', option: 'completed', expected: 'completed' },
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

async function addDuplicateParticipantNames(page: Page, renameMembers = false): Promise<void> {
	await page.route('**/task/api/state**', async (route) => {
		const response = await route.fetch();
		const state: unknown = await response.json();
		if (!isUnknownRecord(state)) throw new Error('flow state response was not an object');
		const members = renameMembers && Array.isArray(state.members)
			? state.members.map((member) => isUnknownRecord(member) && ['kim-intern', 'engineer'].includes(String(member.id))
				? { ...member, name: '이샘플' }
				: member)
			: state.members;
		const tasks = Array.isArray(state.tasks)
			? state.tasks.map((task) => isUnknownRecord(task) && task.id === taskDashboardTaskID
				? { ...task, ownerName: '이샘플', participantNames: ['이샘플', '이샘플'] }
				: task)
			: [];
		await route.fulfill({ response, json: { ...state, members, tasks } });
	});
}

async function addRequesterProvenance(page: Page): Promise<void> {
	await page.route('**/task/api/state**', async (route) => {
		const response = await route.fetch();
		const state: unknown = await response.json();
		if (!isUnknownRecord(state)) throw new Error('flow state response was not an object');
		const tasks = Array.isArray(state.tasks)
			? state.tasks.map((task) => isUnknownRecord(task) && task.id === requestedTaskID
				? { ...task, requesterID: 'requester-1', requesterName: '박예시' }
				: task)
			: [];
		await route.fulfill({ response, json: { ...state, source: 'supabase', tasks } });
	});
}

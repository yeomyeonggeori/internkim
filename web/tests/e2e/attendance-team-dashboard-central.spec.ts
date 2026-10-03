import { expect, test } from '@playwright/test';
import { signInToAttendance } from './attendance-central-test-utils';
import { signInToTheTaskBoard } from './task-central-test-utils';

test.use({ locale: 'ko-KR' });

test('the default dashboard renders bounded team cards without all-company monthly history', async ({ page }) => {
	await signInToTheTaskBoard(page);
	const requested: string[] = [];
	const attendanceInputs: Record<string, unknown>[] = [];
	let teamAnswerBytes = 0;
	page.on('request', (request) => {
		if (request.url().includes('/api/v1/tools/')) requested.push(new URL(request.url()).pathname);
		if (request.url().endsWith('/attendance_list/invoke')) {
			attendanceInputs.push((request.postDataJSON() as { input: Record<string, unknown> }).input);
		}
	});
	page.on('response', async (response) => {
		if (response.url().endsWith('/attendance_team_page_get/invoke') && response.status() === 200) {
			teamAnswerBytes = (await response.body()).byteLength;
		}
	});
	const began = Date.now();
	await page.goto('/example-co/attendance');
	await expect(page.getByTestId('attendance-team-dashboard')).toBeVisible();
	await expect(page.getByTestId('attendance-team-card')).toHaveCount(3);
	await expect(page.getByTestId('team-status-grid')).toHaveCount(0);
	expect(requested).toContain('/api/v1/tools/attendance_team_page_get/invoke');
	expect(attendanceInputs.every((input) => input.scope !== 'all')).toBe(true);
	expect(teamAnswerBytes).toBeGreaterThan(0);
	expect(teamAnswerBytes).toBeLessThan(20_000);
	console.log(JSON.stringify({ teamCardsReadyMilliseconds: Date.now() - began, teamAnswerBytes }));
});

test('the personal month chart and recorded total use only the requester history', async ({ page }) => {
	const historyInputs: Record<string, unknown>[] = [];
	page.on('request', (request) => {
		if (request.url().endsWith('/attendance_list/invoke')) {
			historyInputs.push((request.postDataJSON() as { input: Record<string, unknown> }).input);
		}
	});
	await signInToAttendance(page);
	await expect(page.getByTestId('personal-tools-panel').getByText('기록 합계')).toBeVisible();
	expect(historyInputs.length).toBeGreaterThan(0);
	expect(historyInputs.every((input) => input.scope !== 'all' && Array.isArray(input.personHints) && input.personHints.length === 1)).toBe(true);
	await expect(page.getByTestId('team-status-grid')).toHaveCount(0);
	const panel = page.getByTestId('personal-tools-panel');
	const firstMonthReads = historyInputs.length;
	await panel.getByRole('button', { name: '이전 달' }).click();
	await expect.poll(() => historyInputs.length).toBe(firstMonthReads + 1);
	await expect(panel.getByText('기록 합계')).toBeVisible();
	await panel.getByRole('button', { name: '다음 달' }).click();
	await expect(panel.getByText('기록 합계')).toBeVisible();
	expect(historyInputs).toHaveLength(firstMonthReads + 1);
});

test('a team opens a server-paged employee list with search and on-demand monthly detail', async ({ page }) => {
	await signInToAttendance(page);
	const development = page.getByTestId('attendance-team-card').filter({ hasText: '개발팀' });
	await expect(development).toBeVisible();
	await development.getByRole('button', { name: '팀 구성원' }).click();
	const employeePage = page.getByTestId('team-employee-page');
	await expect(employeePage).toBeVisible();
	await expect(employeePage.getByText('이샘플')).toBeVisible();
	expect(await employeePage.locator('button').count()).toBeLessThanOrEqual(24);

	await page.getByRole('textbox', { name: '이름 또는 이메일 검색' }).fill('없는구성원');
	await expect(employeePage.getByText('표시할 구성원이 없습니다.')).toBeVisible();
	await page.getByRole('textbox', { name: '이름 또는 이메일 검색' }).fill('이샘플');
	await expect(employeePage.getByText('이샘플')).toBeVisible();
	await employeePage.getByRole('button', { name: /이샘플/ }).click();
	await expect(page.getByRole('dialog').getByText('이샘플')).toBeVisible();
	const monthReads: Array<{ path: string; input: Record<string, unknown> }> = [];
	page.on('request', (request) => {
		const path = new URL(request.url()).pathname;
		if (path.endsWith('/attendance_list/invoke') || path.endsWith('/leave_list/invoke')) {
			monthReads.push({ path, input: (request.postDataJSON() as { input: Record<string, unknown> }).input });
		}
	});
	await page.getByRole('dialog').getByRole('button', { name: '월간 현황 보기' }).click();
	await expect(page.getByTestId('team-status-grid')).toBeVisible();
	const attendanceReads = monthReads.filter((read) => read.path.endsWith('/attendance_list/invoke'));
	expect(attendanceReads).toHaveLength(1);
	expect(attendanceReads[0].input.personHints).toHaveLength(1);
	expect(attendanceReads[0].input.scope).toBeUndefined();
	const leaveReads = monthReads.filter((read) => read.path.endsWith('/leave_list/invoke'));
	expect(leaveReads).toHaveLength(1);
	expect(leaveReads[0].input.personHints).toEqual(attendanceReads[0].input.personHints);
});

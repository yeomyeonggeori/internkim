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
		if (response.url().endsWith('/attendance_team_dashboard_get/invoke') && response.status() === 200) {
			teamAnswerBytes = (await response.body()).byteLength;
		}
	});
	const began = Date.now();
	await page.goto('/example-co/attendance');
	await expect(page.getByTestId('attendance-team-dashboard')).toBeVisible();
	await expect(page.getByTestId('attendance-team-card')).toHaveCount(3);
	await expect(page.getByTestId('team-status-grid')).toHaveCount(0);
	expect(requested).toContain('/api/v1/tools/attendance_team_dashboard_get/invoke');
	expect(attendanceInputs.every((input) => input.scope !== 'all')).toBe(true);
	expect(teamAnswerBytes).toBeGreaterThan(0);
	expect(teamAnswerBytes).toBeLessThan(20_000);
	console.log(JSON.stringify({ teamCardsReadyMilliseconds: Date.now() - began, teamAnswerBytes }));
});

test('the own person month opens directly with one scoped history read',async({page})=>{
 const historyInputs:Record<string,unknown>[]=[];
 page.on('request',request=>{if(request.url().endsWith('/attendance_list/invoke'))historyInputs.push((request.postDataJSON() as {input:Record<string,unknown>}).input);});
 await signInToAttendance(page);
 const initialReads=historyInputs.length;
 expect(historyInputs.every(input=>input.scope!=='all'&&Array.isArray(input.personHints)&&input.personHints.length===1)).toBe(true);
 await page.getByTestId('attendance-own-strip').getByRole('button').first().click();
 await expect(page.getByTestId('team-status-grid')).toBeVisible();
 expect(historyInputs).toHaveLength(initialReads+1);
 expect(historyInputs.every(input=>input.scope!=='all'&&Array.isArray(input.personHints)&&input.personHints.length===1)).toBe(true);
 const before=historyInputs.length;
 await page.getByRole('dialog').getByRole('button',{name:'이전 달',exact:true}).click();
 await expect.poll(()=>historyInputs.length).toBe(before+1);
});

test('a team opens a server-paged employee list with search and on-demand monthly detail', async ({ page }) => {
	await signInToAttendance(page);
	const development = page.getByTestId('attendance-team-card').filter({ hasText: '개발팀' });
	await expect(development).toBeVisible();
	await development.getByRole('button', { name: '구성원 보기' }).click();
	const employeePage = page.getByTestId('team-employee-page');
	await expect(employeePage).toBeVisible();
	await expect(employeePage.getByText('이샘플')).toBeVisible();
	expect(await employeePage.getByRole('button',{name:/이샘플/}).count()).toBe(1);

	await page.getByRole('textbox', { name: '이름 또는 이메일 검색' }).fill('없는구성원');
	await expect(employeePage.getByText('표시할 구성원이 없습니다.')).toBeVisible();
	await page.getByRole('textbox', { name: '이름 또는 이메일 검색' }).fill('이샘플');
	await expect(employeePage.getByText('이샘플')).toBeVisible();
	const monthReads: Array<{ path: string; input: Record<string, unknown> }> = [];
	page.on('request', (request) => {
		const path = new URL(request.url()).pathname;
		if (path.endsWith('/attendance_list/invoke') || path.endsWith('/leave_list/invoke')) {
			monthReads.push({ path, input: (request.postDataJSON() as { input: Record<string, unknown> }).input });
		}
	});
	await employeePage.getByRole('button', { name: /이샘플/ }).click();
	await expect(page.getByTestId('team-status-grid')).toBeVisible();
	const attendanceReads = monthReads.filter((read) => read.path.endsWith('/attendance_list/invoke'));
	expect(attendanceReads).toHaveLength(1);
	expect(attendanceReads[0].input.personHints).toHaveLength(1);
	expect(attendanceReads[0].input.scope).toBeUndefined();
	const leaveReads = monthReads.filter((read) => read.path.endsWith('/leave_list/invoke'));
	expect(leaveReads).toHaveLength(1);
	expect(leaveReads[0].input.personHints).toEqual(attendanceReads[0].input.personHints);
});

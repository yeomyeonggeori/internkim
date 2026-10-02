import { taskWeekMondayForCode } from '../../src/lib/task/task-week-code';
import { member1ID } from './central-test-utils';
import { expect, test } from '@playwright/test';
import { signInToTheTaskBoard, seedTasks, removeTasks, taskCard } from './task-central-test-utils';
import { signInToCalendar, seedCalendarEvents, cleanupCalendarEvents } from './calendar-central-test-utils';

test.use({ locale: 'ko-KR' });

for (const width of [320, 390]) {
	test(`calendar compact controls keep authenticated reads usable at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 844 });
		await signInToCalendar(page);
		await expect(page.locator('.calendar-stage')).toBeVisible();
		await expect(page.getByText('this page is not served by the central plane', { exact: true })).toBeHidden();
		await expect(page.locator('.calendar-toolbar').getByRole('button', { name: '오늘', exact: true })).toBeVisible();
		const output = process.env.MOBILE_UX_SCREENSHOTS;
		if (output) await page.screenshot({ path: `${output}/calendar-compact-${width}.png` });
		await page.getByRole('button', { name: '일정 도구', exact: true }).click();
		await expect(page.getByRole('combobox', { name: '참여자 선택', exact: true })).toBeVisible();
		if (output) await page.screenshot({ path: `${output}/calendar-tools-${width}.png` });
		await page.getByRole('button', { name: '설정', exact: true }).click();
		await expect(page.getByRole('dialog')).toBeVisible();
	});
}

test('task startup leaves attendance reads to the clock menu', async ({ page }) => {
	const attendanceReads: string[] = [];
	page.on('request', request => {
		const path = new URL(request.url()).pathname;
		if (/\/(company_settings_get|attendance_list|leave_list)\/invoke$/.test(path)) attendanceReads.push(path);
	});
	await signInToTheTaskBoard(page);
	await page.waitForTimeout(500);
	expect(attendanceReads).toEqual([]);
	await page.keyboard.press('.');
	await expect(page.locator('[data-app-rail-profile-menu]')).toBeVisible();
	await expect.poll(() => attendanceReads.some(path => path.endsWith('/attendance_list/invoke'))).toBe(true);
	await expect(page.locator('[data-app-rail-profile-menu]').getByRole('menuitem', { name: /출근|퇴근/ }).first()).toBeVisible();
	await page.keyboard.press('Escape');
	await page.keyboard.press('/');
	await expect(page.getByRole('dialog').getByRole('option', { name: /출근|퇴근/ }).first()).toBeVisible();
});

test('a cold command palette loads its clock actions', async ({ page }) => {
	let attendanceReads = 0;
	page.on('request', request => {
		if (new URL(request.url()).pathname.endsWith('/attendance_list/invoke')) attendanceReads++;
	});
	await signInToTheTaskBoard(page);
	expect(attendanceReads).toBe(0);
	await page.keyboard.press('/');
	await expect.poll(() => attendanceReads).toBe(1);
	await expect(page.getByRole('dialog').getByRole('option', { name: /출근 ·/ }).first()).toBeVisible();
});

test('calendar search changes dates both inside the calendar and from another route', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-06-15T12:00:00+09:00'));
	await page.route('**/api/calendar/holidays?**', route => route.fulfill({ json: { holidays: [], degraded: false } }));
	const [eventID] = await seedCalendarEvents([{ title: '검색으로 이동하는 팔월', startISO: '2026-08-12T09:00:00+09:00', endISO: '2026-08-12T10:00:00+09:00' }]);
	try {
		await signInToCalendar(page);
		for (const fromTask of [false, true]) {
			if (fromTask) {
				await page.goto('/example-co/task');
				await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
			}
			await page.keyboard.press('/');
			await page.getByPlaceholder('검색').fill('검색으로 이동하는 팔월');
			await page.getByRole('option').filter({ hasText: '검색으로 이동하는 팔월' }).first().click();
			await expect(page.locator('.calendar-toolbar-title')).toContainText('8월');
			await expect(page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first()).toBeVisible();
		}
	} finally { await cleanupCalendarEvents([eventID]); }
});

test('calendar keys do not mutate an event behind the settings sheet', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-06-15T12:00:00+09:00'));
	await page.route('**/api/calendar/holidays?**', route => route.fulfill({ json: { holidays: [], degraded: false } }));
	await page.route('**/api/calendar/subscription', route => route.fulfill({ json: { address: 'https://example.invalid/calendar.ics', registered: true } }));
	const [eventID] = await seedCalendarEvents([{ title: '설정 뒤에서 유지하는 일정', startISO: '2026-06-15T09:00:00+09:00', endISO: '2026-06-15T10:00:00+09:00' }]);
	try {
		await signInToCalendar(page);
		await page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first().click();
		await page.getByRole('button', { name: '설정', exact: true }).click();
		const dialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: '설정' }) });
		await expect(dialog).toBeVisible();
		await dialog.getByRole('button', { name: 'Copy', exact: true }).focus();
		let mutations = 0;
		page.on('request', request => { if (request.url().includes('/event_') && request.url().includes('/invoke') && !request.url().includes('/event_list/')) mutations++; });
		for (const key of ['Delete', 'Backspace', 'Enter', 'Control+z', 'ArrowRight']) await page.keyboard.press(key);
		await page.keyboard.press('Escape');
		await page.reload();
		await expect(page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first()).toBeVisible();
		expect(mutations).toBe(0);
	} finally { await cleanupCalendarEvents([eventID]); }
});

test('week changes reuse state and survive session refresh', async ({ page }) => {
	await signInToTheTaskBoard(page);
	let taskReads = 0;
	page.on('request', request => { if (request.url().includes('/task_list/invoke')) taskReads++; });
	await page.getByRole('button', { name: '이전 주', exact: true }).click();
	const selectedWeek = new URL(page.url()).searchParams.get('week');
	expect(taskReads).toBe(0);
	await page.evaluate(() => window.dispatchEvent(new Event('focus')));
	await expect.poll(() => taskReads).toBeGreaterThan(0);
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	expect(new URL(page.url()).searchParams.get('week')).toBe(selectedWeek);
});

test('a delayed refresh accepts new state while keeping a locally chosen week', async ({ page }) => {
	await signInToTheTaskBoard(page);
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	let started!: () => void;
	const pending = new Promise<void>(resolve => { started = resolve; });
	await page.route('**/task_list/invoke', async route => { started(); await gate; await route.continue(); });
	await page.keyboard.press('r');
	await pending;
	await page.getByRole('button', { name: '이전 주', exact: true }).click();
	const selectedWeek = new URL(page.url()).searchParams.get('week');
	const selectedMonday = taskWeekMondayForCode(selectedWeek ?? '')?.toISOString().slice(0, 10);
	if (!selectedMonday) throw new Error('The selected week has no Monday');
	const [taskID] = await seedTasks([{ title: '늦게 도착하는 새 업무', status: 'completed', participantIDs: [member1ID], startsAtISO: `${selectedMonday}T12:00:00+09:00`, endsAtISO: `${selectedMonday}T13:00:00+09:00` }]);
	try {
		release();
		await expect(taskCard(page, taskID)).toBeVisible();
		expect(new URL(page.url()).searchParams.get('week')).toBe(selectedWeek);
	} finally { release(); await removeTasks([taskID]); }
});

test('a late task response cannot change the route after leaving', async ({ page }) => {
	await signInToTheTaskBoard(page);
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	let started!: () => void;
	const pending = new Promise<void>(resolve => { started = resolve; });
	await page.route('**/task_list/invoke', async route => { started(); await gate; await route.continue(); });
	await page.keyboard.press('r');
	await pending;
	await page.locator('a[href="/example-co/organization"]').first().click();
	await expect(page.getByTestId('organization-board')).toBeVisible();
	const response = page.waitForResponse(response => response.url().includes('/task_list/invoke'));
	release();
	await response;
	await page.waitForTimeout(150);
	expect(new URL(page.url()).searchParams.has('week')).toBe(false);
});

test('an invalidated admin prefetch cannot leave the next selected view empty', async ({ page }) => {
	const { seedLeave, cleanupLeave, member3ID, member3Name } = await import('./central-test-utils');
	const { signInToAttendance } = await import('./attendance-central-test-utils');
	const [leaveID] = await seedLeave([{ memberID: member3ID, kind: 'annual', days: 1, status: 'requested', startISO: '2026-10-15T00:00:00+09:00', endISO: '2026-10-16T00:00:00+09:00', note: '선읽기 무효화 회귀' }]);
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	let started!: () => void;
	const pending = new Promise<void>(resolve => { started = resolve; });
	let allBalanceReads = 0;
	try {
		await signInToAttendance(page);
		await page.route('**/leave_balance/invoke', async route => {
			if (route.request().postDataJSON()?.input?.scope === 'all' && ++allBalanceReads === 1) { started(); await gate; }
			await route.continue();
		});
		await page.getByTestId('leave-approval-navigation').click();
		const card = page.getByTestId(`leave-approval-request-${leaveID}`);
		await expect(card).toBeVisible();
		await page.getByTestId('leave-management-navigation').hover();
		await pending;
		await card.getByRole('button', { name: '승인', exact: true }).click();
		await expect(card).toHaveCount(0);
		await page.getByTestId('leave-management-navigation').click();
		await expect.poll(() => allBalanceReads).toBe(2);
		release();
		await expect(page.getByTestId('leave-management-view').getByText(member3Name, { exact: true }).first()).toBeVisible();
		await expect(page.getByTestId('leave-management-refresh')).toBeEnabled();
	} finally { release(); await cleanupLeave([leaveID]); }
});

test('changing account scope removes the previous task editor draft', async ({ page, context }) => {
	const { openTaskCard, taskSheet } = await import('./task-central-test-utils');
	const [taskID] = await seedTasks([{ title: '이전 계정의 열린 draft', status: 'planned', participantIDs: [member1ID] }]);
	try {
		await signInToTheTaskBoard(page);
		await openTaskCard(page, taskID);
		await expect(taskSheet(page)).toBeVisible();
		const response = await page.request.post(`${process.env.SUPABASE_URL}/auth/v1/token?grant_type=password`, { headers: { apikey: process.env.SUPABASE_PUBLISHABLE_KEY ?? '' }, data: { email: 'member2@example.com', password: 'seed-password' } });
		expect(response.ok()).toBe(true);
		const session = await response.json();
		const storageKey = `sb-${new URL(process.env.SUPABASE_URL ?? '').hostname.split('.')[0]}-auth-token`;
		const otherTab = await context.newPage();
		await otherTab.goto('/example-co/organization');
		await otherTab.evaluate(({ storageKey, session }) => {
			localStorage.setItem(storageKey, JSON.stringify(session));
			const channel = new BroadcastChannel(storageKey);
			channel.postMessage({ event: 'SIGNED_IN', session });
			channel.close();
		}, { storageKey, session });
		await page.bringToFront();
		await page.evaluate(() => window.dispatchEvent(new Event('focus')));
		await expect(taskSheet(page)).toHaveCount(0);
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	} finally { await removeTasks([taskID]); }
});

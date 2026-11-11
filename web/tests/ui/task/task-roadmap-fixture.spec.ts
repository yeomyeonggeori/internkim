import { expect, type Locator, type Page, type Route, test } from '@playwright/test';
import type { RecordTask, RecordTaskList } from '../../../src/lib/task/task-record';
import { taskWeekOfDate } from '../../../src/lib/task/task-week-code';
import { taskSheet } from './task-central-test-utils';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const parentID = '30000000-0000-4000-8000-000000000011';
const childID = '30000000-0000-4000-8000-000000000012';
const siblingID = '30000000-0000-4000-8000-000000000013';
const strangerID = '30000000-0000-4000-8000-000000000014';
const undatedID = '30000000-0000-4000-8000-000000000015';
const completedID = '30000000-0000-4000-8000-000000000016';
const oldCompletedID = '30000000-0000-4000-8000-000000000017';
const laterID = '30000000-0000-4000-8000-000000000018';
const businessName = 'Fixture 사업';
const today = new Date('2026-11-11T12:00:00+09:00');
const holidayISO = '2026-11-10';

function november(day: number): string {
	return `2026-11-${String(day).padStart(2, '0')}`;
}

function taskFixture(taskID: string, content: string, fields: Partial<RecordTask>): RecordTask {
	return {
		taskID, content, status: 'in_progress', size: 'M', ownerID: memberID,
		ownerName: '이샘플', participantIDs: [memberID], participantNames: ['이샘플'],
		createdAt: today.toISOString(), weekCode: taskWeekOfDate(today).code,
		canUpdate: true, canDelete: true, ...fields
	};
}

type TaskDateWrite = { taskHint: string; startsAt?: string; endsAt?: string };

class TaskRoadmapFixture {
	tasks = [
		taskFixture(parentID, 'Fixture 로드맵 부모 업무', { startDate: november(2), endDate: november(27) }),
		taskFixture(childID, 'Fixture 로드맵 자녀 업무', { parentTaskID: parentID, startDate: november(9), endDate: november(12) }),
		taskFixture(siblingID, 'Fixture 로드맵 형제 업무', { parentTaskID: parentID, status: 'planned', startDate: november(13), endDate: november(18) }),
		taskFixture(strangerID, 'Fixture 로드맵 관계없는 업무', { business: businessName, startDate: november(10), endDate: november(13) }),
		taskFixture(completedID, 'Fixture 로드맵 완료 업무', { status: 'completed', startDate: november(9), endDate: november(11) }),
		taskFixture(oldCompletedID, 'Fixture 로드맵 지난 완료 업무', { status: 'completed', startDate: november(1), endDate: november(4) }),
		taskFixture(laterID, 'Fixture 로드맵 나중 업무', { status: 'planned', startDate: november(24), endDate: november(26) }),
		taskFixture(undatedID, 'Fixture 로드맵 날짜 없는 업무', { status: 'planned' })
	];
	writes: TaskDateWrite[] = [];
	heldList: Promise<void> | null = null;

	listed(): RecordTaskList {
		return {
			scope: 'all', count: this.tasks.length, tasks: this.tasks,
			registeredLabels: { businesses: [{ name: businessName }], types: [], sizes: ['S', 'M', 'L'], statuses: ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'] }
		};
	}

	async answer(route: Route): Promise<void> {
		const toolName = new URL(route.request().url()).pathname.split('/').at(-2);
		if (toolName === 'person_list') {
			await route.fulfill({ json: { result: { requesterID: memberID, count: 1, people: [{ personID: memberID, name: '이샘플', email: 'member@example.com', isAdmin: true, hireDate: '2026-01-01' }] } } });
			return;
		}
		if (toolName === 'task_list' && this.heldList) await this.heldList;
		if (toolName === 'task_list' || toolName === 'task_board_get') {
			await route.fulfill({ json: { result: this.listed() } });
			return;
		}
		if (toolName === 'company_settings_get') {
			await route.fulfill({ json: { result: { timeZone: 'Asia/Seoul' } } });
			return;
		}
		if (toolName === 'task_update') {
			const written: { input: TaskDateWrite } = route.request().postDataJSON();
			this.writes.push(written.input);
			const task = this.tasks.find(candidate => candidate.taskID === written.input.taskHint);
			if (!task) throw new Error('Fixture write must target a seeded task');
			if (written.input.startsAt !== undefined) task.startDate = written.input.startsAt;
			if (written.input.endsAt !== undefined) task.endDate = written.input.endsAt;
			await route.fulfill({ json: { result: task } });
			return;
		}
		await route.fulfill({ status: 503, json: { error: 'Tool has no supplementary UI fixture' } });
	}
}

async function dragBarByDays(page: Page, roadmap: Locator, taskID: string, days: number): Promise<void> {
	const box = await roadmap.locator(`[data-roadmap-bar="${taskID}"]`).boundingBox();
	const dayWidth = await roadmap.locator('[data-roadmap-today]').evaluate(element => element.parentElement?.getBoundingClientRect().width ?? 0);
	if (!box || dayWidth === 0) throw new Error('The roadmap draws the bar and its day columns');
	await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
	await page.mouse.down();
	await page.mouse.move(box.x + box.width / 2 + dayWidth * days, box.y + box.height / 2, { steps: 8 });
	await page.mouse.up();
}

async function openRoadmap(page: Page, fixture: TaskRoadmapFixture): Promise<Locator> {
	await page.clock.setFixedTime(today);
	await page.addInitScript(({ memberID }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({
			access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`,
			refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600,
			user: { id: memberID, email: 'member@example.com', aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() }
		}));
	}, { memberID });
	await page.route('http://127.0.0.1:56801/**', async route => {
		if (new URL(route.request().url()).pathname === '/rest/v1/member') {
			await route.fulfill({ json: { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } });
			return;
		}
		await route.fulfill({ json: [] });
	});
	await page.route('**/api/member/me', route => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/agent/api/**', route => route.fulfill({ json: {} }));
	await page.route('**/api/v1/tools/*/invoke', route => fixture.answer(route));
	await page.route('**/api/calendar/holidays**', route => route.fulfill({ json: { holidays: [{ id: 'holiday:holiday_api:kr:fixture', title: 'Fixture 공휴일', date: holidayISO, source: 'holiday_api', readOnly: true, color: '#dc2626' }] } }));
	await page.goto('/example-co/task');
	return showRoadmapTab(page);
}

async function showRoadmapTab(page: Page): Promise<Locator> {
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await page.getByRole('tab', { name: '로드맵' }).click();
	const roadmap = page.locator('[data-task-roadmap]');
	await expect(roadmap).toBeVisible();
	return roadmap;
}

function roadmapRow(roadmap: Locator, taskID: string): Locator {
	return roadmap.locator(`[data-roadmap-task="${taskID}"]`);
}


function shownDays(roadmap: Locator): Locator {
	return roadmap.locator('[data-roadmap-day]');
}

async function firstShownDay(roadmap: Locator): Promise<string | null> {
	return shownDays(roadmap).first().getAttribute('data-roadmap-day');
}

test.use({ locale: 'ko-KR' });

test('supplementary fixture: a wide screen shows two weeks from this Sunday with no sideways scroll', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(shownDays(roadmap)).toHaveCount(14);
	expect(await firstShownDay(roadmap)).toBe(november(8));
	await expect(shownDays(roadmap).last()).toHaveAttribute('data-roadmap-day', november(21));
	expect(await roadmap.evaluate(element => element.scrollWidth - element.clientWidth)).toBe(0);
	await expect(page.getByRole('button', { name: '월 선택' })).toContainText('11월');
	await expect(page.getByRole('button', { name: '날짜로 주차 이동' })).toHaveCount(0);
});

test('supplementary fixture: a phone shows one week, and the type is written inside the bar', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(shownDays(roadmap)).toHaveCount(7);
	expect(await firstShownDay(roadmap)).toBe(november(8));
	expect(await roadmap.evaluate(element => element.scrollWidth - element.clientWidth)).toBe(0);
	await expect(roadmapRow(roadmap, childID).getByText('기타', { exact: true }).filter({ visible: true })).toHaveCount(1);
	await page.getByRole('button', { name: '다음 주' }).click();
	expect(await firstShownDay(roadmap)).toBe(november(15));
});

test('supplementary fixture: the week arrows move the window seven days at a time', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await page.getByRole('button', { name: '다음 주' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(15));
	await page.getByRole('button', { name: '이전 주' }).click();
	await page.getByRole('button', { name: '이전 주' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(1));
});

test('supplementary fixture: a sideways wheel turns one week however long it keeps going', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const box = await roadmap.boundingBox();
	if (!box) throw new Error('The roadmap is drawn');
	await page.mouse.move(box.x + box.width / 2, box.y + 120);
	for (let step = 0; step < 6; step += 1) await page.mouse.wheel(60, 0);
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(15));
	await page.waitForTimeout(400);
	await page.mouse.wheel(-120, 0);
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(8));
});

test('supplementary fixture: choosing a date in the header makes it the first day', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await roadmap.locator(`[data-roadmap-day="${november(12)}"]`).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(12));
	await expect(shownDays(roadmap).last()).toHaveAttribute('data-roadmap-day', november(25));
});

test('supplementary fixture: the month selector opens the week of the first and follows the window', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const monthButton = page.getByRole('button', { name: '월 선택' });
	await page.getByRole('button', { name: '다음 달' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(29));
	await expect(roadmap.locator('[data-roadmap-day="2026-12-01"]')).toContainText('12/1');
	await expect(monthButton).toContainText('12월');
	await page.getByRole('button', { name: '이전 주' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(22));
	await expect(monthButton).toContainText('11월');
	await page.getByRole('button', { name: '이전 달' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', '2026-10-04');
	await expect(monthButton).toContainText('10월');
	await monthButton.click();
	await page.getByRole('option', { name: '이번 달' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(8));
});

test('supplementary fixture: the window stops six months from this month', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const monthButton = page.getByRole('button', { name: '월 선택' });
	await monthButton.click();
	await page.getByRole('option', { name: '2027년 5월' }).click();
	await expect(monthButton).toContainText('2027년 5월');
	for (let week = 0; week < 6; week += 1) {
		const next = page.getByRole('button', { name: '다음 주' });
		if (await next.isDisabled()) break;
		await next.click();
	}
	await expect(page.getByRole('button', { name: '다음 주' })).toBeDisabled();
	await expect(monthButton).toContainText('2027년 5월');
	expect(await firstShownDay(roadmap)).toBe('2027-05-23');
});

test('supplementary fixture: holidays and weekends are red, and child progress shows on parents only', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(roadmap.locator('[title="Fixture 공휴일"]')).toHaveAttribute('data-roadmap-red-day', 'true');
	await expect(roadmap.locator(`[data-roadmap-day="${november(8)}"]`)).toHaveAttribute('data-roadmap-red-day', 'true');
	await expect(roadmap.locator(`[data-roadmap-day="${november(14)}"]`)).toHaveAttribute('data-roadmap-red-day', 'true');
	await expect(roadmap.locator(`[data-roadmap-day="${november(12)}"]`)).not.toHaveAttribute('data-roadmap-red-day', 'true');
	await expect(roadmap.locator(`[data-roadmap-day="${november(10)}"]`)).toHaveCSS('color', await roadmap.locator(`[data-roadmap-day="${november(8)}"]`).evaluate(element => getComputedStyle(element).color));
	await expect(roadmapRow(roadmap, parentID).locator('[data-roadmap-progress]')).toContainText('자녀 업무 0 / 2 완료');
	await expect(roadmapRow(roadmap, childID).locator('[data-roadmap-progress]')).toHaveCount(0);
	await expect(roadmapRow(roadmap, parentID).locator('[data-roadmap-parent-icon]')).toHaveCount(1);
});

test('supplementary fixture: tasks are one list in start order, with no status groups and no title inside a bar', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const order = await roadmap.locator('[data-roadmap-task]').evaluateAll(rows => rows.map(row => row.getAttribute('data-roadmap-task')));
	expect(order).toEqual([oldCompletedID, parentID, childID, completedID, strangerID, siblingID, laterID, undatedID]);
	await expect(roadmap.getByRole('button', { name: '진행', exact: true })).toHaveCount(0);
	await expect(roadmap.locator(`[data-roadmap-bar="${childID}"]`)).toHaveText('');
	await expect(roadmapRow(roadmap, childID).getByRole('img', { name: '진행' })).toBeVisible();
});

test('supplementary fixture: completed work outside the window keeps its row and says when it was', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(roadmap.locator(`[data-roadmap-bar="${completedID}"]`)).toBeVisible();
	await expect(roadmap.locator(`[data-roadmap-bar="${oldCompletedID}"]`)).toHaveCount(0);
	await expect(roadmap.locator(`[data-roadmap-away="${oldCompletedID}"]`)).toHaveText('11/1 – 11/4');
	await page.getByRole('button', { name: '이전 주' }).click();
	await expect(roadmap.locator(`[data-roadmap-bar="${oldCompletedID}"]`)).toBeVisible();
});

test('supplementary fixture: add-dates waits for the pointer', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const addDates = roadmap.locator(`[data-roadmap-add-dates="${undatedID}"]`);
	await expect(addDates).toHaveCSS('opacity', '0');
	await roadmapRow(roadmap, undatedID).hover();
	await expect(addDates).toHaveCSS('opacity', '1');
});

test('supplementary fixture: pressing a bar leaves it where it is', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const bar = roadmap.locator(`[data-roadmap-bar="${strangerID}"]`);
	const before = await bar.boundingBox();
	if (!before) throw new Error('The roadmap draws the bar');
	await page.mouse.move(before.x + before.width / 2, before.y + before.height / 2);
	await page.mouse.down();
	const pressed = await bar.boundingBox();
	await page.mouse.up();
	expect(pressed?.y).toBe(before.y);
});

test('supplementary fixture: the first click on a bar chooses it and marks its days, the second opens it', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const bar = roadmap.locator(`[data-roadmap-bar="${childID}"]`);
	await bar.click();
	await expect(bar).toHaveAttribute('aria-pressed', 'true');
	await expect(roadmap.locator('[data-roadmap-selected-span]')).toBeVisible();
	await expect(roadmapRow(roadmap, strangerID)).toHaveAttribute('data-roadmap-dimmed', 'true');
	for (const relatedID of [parentID, childID, siblingID]) await expect(roadmapRow(roadmap, relatedID)).not.toHaveAttribute('data-roadmap-dimmed', 'true');
	await expect(taskSheet(page)).not.toBeVisible();
	await bar.click();
	await expect(taskSheet(page)).toBeVisible();
});

test('supplementary fixture: a task with no relations is chosen first too, and dims nothing', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await roadmap.locator(`[data-roadmap-bar="${strangerID}"]`).click();
	await expect(taskSheet(page)).not.toBeVisible();
	await expect(roadmapRow(roadmap, parentID)).not.toHaveAttribute('data-roadmap-dimmed', 'true');
	await roadmap.locator(`[data-roadmap-bar="${strangerID}"]`).click();
	await expect(taskSheet(page)).toBeVisible();
});

test('supplementary fixture: clicking outside the tasks lets the choice go', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const bar = roadmap.locator(`[data-roadmap-bar="${childID}"]`);
	await bar.click();
	await expect(bar).toHaveAttribute('aria-pressed', 'true');
	await page.getByRole('tab', { name: '로드맵' }).click();
	await expect(bar).toHaveAttribute('aria-pressed', 'false');
	await expect(roadmap.locator('[data-roadmap-selected-span]')).toHaveCount(0);
});

test('supplementary fixture: a task title opens the task on the first click', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await roadmap.locator(`[data-roadmap-title="${childID}"]`).click();
	await expect(taskSheet(page)).toBeVisible();
});

test('supplementary fixture: the dates of a task outside the window bring its bar into view', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const away = roadmap.locator(`[data-roadmap-away="${laterID}"]`);
	await expect(away).toHaveText('11/24 – 11/26');
	await away.click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(22));
	await expect(roadmap.locator(`[data-roadmap-bar="${laterID}"]`)).toHaveAttribute('aria-pressed', 'true');
	await expect(taskSheet(page)).not.toBeVisible();
});

test('supplementary fixture: the date at either end of a cut bar moves to that end', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const parentRow = roadmapRow(roadmap, parentID);
	await expect(parentRow.getByRole('button', { name: '11/2로 이동' })).toHaveText('11/2');
	await parentRow.getByRole('button', { name: '11/27로 이동' }).click();
	await expect(shownDays(roadmap).first()).toHaveAttribute('data-roadmap-day', november(22));
	await expect(roadmap.locator(`[data-roadmap-bar="${parentID}"]`)).toHaveAttribute('aria-pressed', 'true');
});

test('supplementary fixture: dragging a bar moves both dates and saves them', async ({ page }) => {
	const fixture = new TaskRoadmapFixture();
	const roadmap = await openRoadmap(page, fixture);
	await dragBarByDays(page, roadmap, childID, 3);
	await expect.poll(() => fixture.writes).toEqual([{ taskHint: childID, startsAt: november(12), endsAt: november(15) }]);
	await expect(taskSheet(page)).not.toBeVisible();
});

test('supplementary fixture: sliding a bar with a finger moves nothing and saves nothing', async ({ page }) => {
	const fixture = new TaskRoadmapFixture();
	const roadmap = await openRoadmap(page, fixture);
	const bar = roadmap.locator(`[data-roadmap-bar="${childID}"]`);
	const before = await bar.boundingBox();
	if (!before) throw new Error('The roadmap draws the bar');
	const touch = { pointerType: 'touch', pointerId: 7, isPrimary: true, button: 0, clientY: before.y + before.height / 2 };
	await bar.dispatchEvent('pointerdown', { ...touch, clientX: before.x + 4 });
	for (const step of [20, 60, 120]) await bar.dispatchEvent('pointermove', { ...touch, clientX: before.x + 4 + step });
	await bar.dispatchEvent('pointerup', { ...touch, clientX: before.x + 124 });
	expect((await bar.boundingBox())?.x).toBe(before.x);
	await dragBarByDays(page, roadmap, childID, 3);
	await expect.poll(() => fixture.writes).toEqual([{ taskHint: childID, startsAt: november(12), endsAt: november(15) }]);
});

test('supplementary fixture: the stored history draws the roadmap first, and bars drag only once the full history arrives', async ({ page }) => {
	const fixture = new TaskRoadmapFixture();
	await openRoadmap(page, fixture);
	let releaseList = () => {};
	fixture.heldList = new Promise(resolve => { releaseList = resolve; });
	await page.reload();
	const roadmap = await showRoadmapTab(page);
	await expect(roadmap.locator(`[data-roadmap-bar="${childID}"]`)).toBeVisible();
	await dragBarByDays(page, roadmap, childID, 3);
	releaseList();
	fixture.heldList = null;
	await expect(roadmap.locator(`[data-roadmap-bar="${childID}"]`)).toHaveClass(/cursor-grab/);
	await dragBarByDays(page, roadmap, childID, 3);
	await expect.poll(() => fixture.writes).toEqual([{ taskHint: childID, startsAt: november(12), endsAt: november(15) }]);
});

test('supplementary fixture: the roadmap business badge is the one the list tab shows', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const roadmapBadge = roadmapRow(roadmap, strangerID).locator('[data-slot="badge"]').filter({ hasText: businessName });
	const roadmapLook = await roadmapBadge.evaluate(element => { const style = getComputedStyle(element); return [style.backgroundColor, style.color, style.borderRadius, style.fontSize, style.height]; });
	await page.getByRole('tab', { name: '표' }).click();
	const listBadge = page.getByRole('tabpanel', { name: '표' }).locator('[data-slot="badge"]').filter({ hasText: businessName }).first();
	await expect(listBadge).toBeVisible();
	const listLook = await listBadge.evaluate(element => { const style = getComputedStyle(element); return [style.backgroundColor, style.color, style.borderRadius, style.fontSize, style.height]; });
	expect(roadmapLook).toEqual(listLook);
});

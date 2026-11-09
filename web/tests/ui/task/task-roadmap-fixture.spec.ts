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
const holidayDay = 10;

function dayOfThisMonth(day: number): string {
	const today = new Date();
	return `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

function taskFixture(taskID: string, content: string, fields: Partial<RecordTask>): RecordTask {
	return {
		taskID, content, status: 'in_progress', size: 'M', ownerID: memberID,
		ownerName: '이샘플', participantIDs: [memberID], participantNames: ['이샘플'],
		createdAt: new Date().toISOString(), weekCode: taskWeekOfDate(new Date()).code,
		canUpdate: true, canDelete: true, ...fields
	};
}

type TaskDateWrite = { taskHint: string; startsAt?: string; endsAt?: string };

class TaskRoadmapFixture {
	tasks = [
		taskFixture(parentID, 'Fixture 로드맵 부모 업무', { startDate: dayOfThisMonth(2), endDate: dayOfThisMonth(20) }),
		taskFixture(childID, 'Fixture 로드맵 자녀 업무', { parentTaskID: parentID, startDate: dayOfThisMonth(5), endDate: dayOfThisMonth(12) }),
		taskFixture(siblingID, 'Fixture 로드맵 형제 업무', { parentTaskID: parentID, status: 'planned', startDate: dayOfThisMonth(13), endDate: dayOfThisMonth(18) }),
		taskFixture(strangerID, 'Fixture 로드맵 관계없는 업무', { startDate: dayOfThisMonth(3), endDate: dayOfThisMonth(7) }),
		taskFixture(undatedID, 'Fixture 로드맵 날짜 없는 업무', { status: 'planned' })
	];
	writes: TaskDateWrite[] = [];

	listed(): RecordTaskList {
		return {
			scope: 'all', count: this.tasks.length, tasks: this.tasks,
			registeredLabels: { businesses: [], types: [], sizes: ['S', 'M', 'L'], statuses: ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'] }
		};
	}

	async answer(route: Route): Promise<void> {
		const toolName = new URL(route.request().url()).pathname.split('/').at(-2);
		if (toolName === 'person_list') {
			await route.fulfill({ json: { result: { requesterID: memberID, count: 1, people: [{ personID: memberID, name: '이샘플', email: 'member@example.com', isAdmin: true, hireDate: '2026-01-01' }] } } });
			return;
		}
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

async function openRoadmap(page: Page, fixture: TaskRoadmapFixture): Promise<Locator> {
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
	await page.route('**/api/calendar/holidays**', route => route.fulfill({ json: { holidays: [{ id: 'holiday:holiday_api:kr:fixture', title: 'Fixture 공휴일', date: dayOfThisMonth(holidayDay), source: 'holiday_api', readOnly: true, color: '#dc2626' }] } }));
	await page.goto('/example-co/task');
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await page.getByRole('tab', { name: '로드맵' }).click();
	const roadmap = page.locator('[data-task-roadmap]');
	await expect(roadmap).toBeVisible();
	return roadmap;
}

function roadmapRow(roadmap: Locator, taskID: string): Locator {
	return roadmap.locator(`[data-roadmap-task="${taskID}"]`);
}

test.use({ locale: 'ko-KR' });

test('supplementary fixture: the roadmap shows a month selector and child progress on parents only', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(page.getByRole('button', { name: '월 선택' })).toBeVisible();
	await expect(page.getByRole('button', { name: '날짜로 주차 이동' })).toHaveCount(0);
	await expect(roadmapRow(roadmap, parentID).locator('[data-roadmap-progress]')).toContainText('자녀 업무 0 / 2 완료');
	await expect(roadmapRow(roadmap, childID).locator('[data-roadmap-progress]')).toHaveCount(0);
	await expect(roadmapRow(roadmap, parentID).locator('[data-roadmap-parent-icon]')).toHaveCount(1);
	await expect(roadmap.locator(`[data-roadmap-bar="${childID}"]`)).toHaveText('Fixture 로드맵 자녀 업무');
});

test('supplementary fixture: holidays and weekends are red, and add-dates waits for the pointer', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(roadmap.locator(`[title="Fixture 공휴일"]`)).toHaveAttribute('data-roadmap-red-day', 'true');
	const addDates = roadmap.locator(`[data-roadmap-add-dates="${undatedID}"]`);
	await expect(addDates).toHaveCSS('opacity', '0');
	await roadmapRow(roadmap, undatedID).hover();
	await expect(addDates).toHaveCSS('opacity', '1');
});

test('supplementary fixture: the month selector and the horizontal scroll follow each other', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	const monthButton = page.getByRole('button', { name: '월 선택' });
	const shownMonth = await monthButton.innerText();
	const scrolledBefore = await roadmap.evaluate(element => element.scrollLeft);
	await page.getByRole('button', { name: '다음 달' }).click();
	await expect.poll(() => roadmap.evaluate(element => element.scrollLeft)).toBeGreaterThan(scrolledBefore);
	const nextMonth = await monthButton.innerText();
	expect(nextMonth).not.toBe(shownMonth);
	await roadmap.evaluate(element => { element.scrollLeft = 0; });
	await expect.poll(() => monthButton.innerText()).not.toBe(nextMonth);
	await expect(page.getByRole('button', { name: '이전 달' })).toBeDisabled();
});

test('supplementary fixture: opening the roadmap late in the month keeps today in view', async ({ page }) => {
	const today = new Date();
	await page.clock.setFixedTime(new Date(today.getFullYear(), today.getMonth(), 28, 12));
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await expect(roadmap.locator('[data-roadmap-today]')).toBeInViewport();
	await expect(page.getByRole('button', { name: '월 선택' })).toContainText(`${today.getMonth() + 1}월`);
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

test('supplementary fixture: choosing a related task dims the others, and choosing it again opens it', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await roadmapRow(roadmap, childID).getByRole('button', { name: /Fixture 로드맵 자녀 업무/ }).first().click();
	await expect(roadmapRow(roadmap, strangerID)).toHaveAttribute('data-roadmap-dimmed', 'true');
	for (const relatedID of [parentID, childID, siblingID]) await expect(roadmapRow(roadmap, relatedID)).not.toHaveAttribute('data-roadmap-dimmed', 'true');
	await expect(taskSheet(page)).not.toBeVisible();
	await roadmap.locator(`[data-roadmap-bar="${childID}"]`).click();
	await expect(taskSheet(page)).toBeVisible();
});

test('supplementary fixture: a task with no relations opens on the first click', async ({ page }) => {
	const roadmap = await openRoadmap(page, new TaskRoadmapFixture());
	await roadmap.locator(`[data-roadmap-bar="${strangerID}"]`).click();
	await expect(taskSheet(page)).toBeVisible();
	await expect(roadmapRow(roadmap, parentID)).not.toHaveAttribute('data-roadmap-dimmed', 'true');
});

test('supplementary fixture: dragging a bar moves both dates and saves them', async ({ page }) => {
	const fixture = new TaskRoadmapFixture();
	const roadmap = await openRoadmap(page, fixture);
	const bar = roadmap.locator(`[data-roadmap-bar="${childID}"]`);
	const box = await bar.boundingBox();
	const dayWidth = await roadmap.locator('[data-roadmap-today]').evaluate(element => element.parentElement?.getBoundingClientRect().width ?? 0);
	if (!box || dayWidth === 0) throw new Error('The roadmap draws the bar and its day columns');
	await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
	await page.mouse.down();
	await page.mouse.move(box.x + box.width / 2 + dayWidth * 3, box.y + box.height / 2, { steps: 8 });
	await page.mouse.up();
	await expect.poll(() => fixture.writes).toEqual([{ taskHint: childID, startsAt: dayOfThisMonth(8), endsAt: dayOfThisMonth(15) }]);
	await expect(taskSheet(page)).not.toBeVisible();
});

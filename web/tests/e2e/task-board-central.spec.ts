import { expect, test } from '@playwright/test';
import { member1ID, member2ID, member3ID, member3Email } from './central-test-utils';
import {
	columnDropZone,
	columnTaskCount,
	dragCardOntoColumn,
	expectTaskStatus,
	openTaskCard,
	removeTasks,
	seedTasks,
	showEveryParticipant,
	signInToTheTaskBoard,
	taskCard,
	taskColumn,
	taskParticipantIDsOf,
	taskRowTitled,
	taskSheet,
	taskStatusOf,
	type CentralTaskStatus
} from './task-central-test-utils';
import { taskText } from '../../src/routes/task/text';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const boardStatuses: CentralTaskStatus[] = ['requested', 'planned', 'in_progress', 'completed', 'paused'];

let seededTaskIDs: string[] = [];
const taskIDByStatus = new Map<CentralTaskStatus, string>();

async function seedForThisRun(tasks: { title: string; status: CentralTaskStatus; participantIDs?: string[] }[]): Promise<string[]> {
	const identifiers = await seedTasks(
		tasks.map((task) => ({
			title: task.title,
			status: task.status,
			participantIDs: task.participantIDs ?? [member1ID],
			business: '사업하나',
			type: '기능',
			size: 'M'
		}))
	);
	seededTaskIDs.push(...identifiers);
	return identifiers;
}

test.beforeAll(async () => {
	const identifiers = await seedTasks(
		boardStatuses.map((status) => ({
			title: `E2E 보드 ${status}`,
			status,
			participantIDs: [member1ID],
			business: '사업하나',
			type: '기능',
			size: 'M',
			...(status === 'requested' ? { requesterID: member2ID } : {})
		}))
	);
	seededTaskIDs.push(...identifiers);
	boardStatuses.forEach((status, index) => taskIDByStatus.set(status, identifiers[index]));
});

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

test('each column carries the tasks the record gives that status', async ({ page }) => {
	await signInToTheTaskBoard(page);

	for (const status of boardStatuses) {
		const column = taskColumn(page, status);
		await expect(column).toBeVisible();
		await expect(column.locator(`[data-task-board-card="${taskIDByStatus.get(status)}"]`)).toBeVisible();
		const renderedCards = await column.locator('[data-task-board-card]').count();
		await expect(columnTaskCount(page, status)).toHaveText(String(renderedCards));
	}

	const plannedID = taskIDByStatus.get('planned') ?? '';
	await expect(taskColumn(page, 'in_progress').locator(`[data-task-board-card="${plannedID}"]`)).toHaveCount(0);
});

test('a card dragged into another column carries that status on the record', async ({ page }) => {
	const [movedTaskID] = await seedForThisRun([{ title: 'E2E 보드 이동 대상', status: 'planned' }]);
	await signInToTheTaskBoard(page);

	await expect(taskColumn(page, 'planned').locator(`[data-task-board-card="${movedTaskID}"]`)).toBeVisible();
	await dragCardOntoColumn(page, movedTaskID, 'in_progress');

	await expect(taskColumn(page, 'in_progress').locator(`[data-task-board-card="${movedTaskID}"]`)).toBeVisible();
	await expectTaskStatus(movedTaskID, 'in_progress');

	await page.reload();
	await expect(taskColumn(page, 'in_progress').locator(`[data-task-board-card="${movedTaskID}"]`)).toBeVisible();
	await expect(taskColumn(page, 'planned').locator(`[data-task-board-card="${movedTaskID}"]`)).toHaveCount(0);
});

test('a card says its move is in flight, and opens again once the record has it', async ({ page }) => {
	const [movedTaskID] = await seedForThisRun([{ title: 'E2E 보드 이동 후 열기', status: 'planned' }]);
	await signInToTheTaskBoard(page);
	await expect(taskColumn(page, 'planned').locator(`[data-task-board-card="${movedTaskID}"]`)).toBeVisible();

	let releaseMove = (): void => {};
	const moveHeld = new Promise<void>((resolve) => {
		releaseMove = resolve;
	});
	await page.route('**/api/v1/tools/task_update/invoke', async (route) => {
		await moveHeld;
		await route.continue();
	});

	await dragCardOntoColumn(page, movedTaskID, 'in_progress');
	await expect(taskCard(page, movedTaskID)).toHaveAttribute('data-task-board-pending', 'true');

	releaseMove();
	await expectTaskStatus(movedTaskID, 'in_progress');
	await expect(taskCard(page, movedTaskID)).toHaveAttribute('data-task-board-pending', 'false');
	await openTaskCard(page, movedTaskID);
});

test('a move that the company channel also announces leaves no load error behind', async ({ page }) => {
	const [movedTaskID] = await seedForThisRun([{ title: 'E2E 보드 이동 알림', status: 'planned' }]);
	let companyChannelJoined = false;
	page.on('websocket', (socket) => {
		socket.on('framereceived', (frame) => {
			const payload = String(frame.payload);
			if (payload.includes('realtime:company:') && payload.includes('phx_reply')) companyChannelJoined = true;
		});
	});
	await signInToTheTaskBoard(page);
	await expect(taskColumn(page, 'planned').locator(`[data-task-board-card="${movedTaskID}"]`)).toBeVisible();
	await expect.poll(() => companyChannelJoined).toBe(true);
	let boardReadsAnswered = 0;
	await page.route('**/api/v1/tools/task_board_get/invoke', async (route) => {
		await new Promise((resolve) => setTimeout(resolve, 1_500));
		await route.continue();
		boardReadsAnswered += 1;
	});

	await dragCardOntoColumn(page, movedTaskID, 'in_progress');
	await expectTaskStatus(movedTaskID, 'in_progress');

	await expect.poll(() => boardReadsAnswered, { timeout: 10_000 }).toBe(2);
	await expect(page.locator('main[data-task-ready="true"]')).toBeVisible();
	await expect(page.getByText(taskText.ko.loadError)).toHaveCount(0);
	await expect(taskColumn(page, 'in_progress').locator(`[data-task-board-card="${movedTaskID}"]`)).toBeVisible();
});

test('a move the record refuses puts the card back and says why', async ({ page }) => {
	const [refusedTaskID] = await seedForThisRun([{ title: 'E2E 보드 거절 대상', status: 'planned' }]);
	await page.route('**/api/v1/tools/task_update/invoke', async (route) => {
		await route.fulfill({
			status: 409,
			contentType: 'application/json',
			body: JSON.stringify({
				error: 'task changed since it was read',
				errorCode: 'record_refused'
			})
		});
	});
	await signInToTheTaskBoard(page);

	await dragCardOntoColumn(page, refusedTaskID, 'in_progress');

	await expect(page.getByText('task changed since it was read')).toBeVisible();
	await expect(taskColumn(page, 'planned').locator(`[data-task-board-card="${refusedTaskID}"]`)).toBeVisible();
	expect(await taskStatusOf(refusedTaskID)).toBe('planned');
});

test('a drop that did not start from a board card writes nothing', async ({ page }) => {
	const [untouchedTaskID] = await seedForThisRun([{ title: 'E2E 보드 외부 드롭 대상', status: 'planned' }]);
	await signInToTheTaskBoard(page);
	await expect(taskColumn(page, 'planned').locator(`[data-task-board-card="${untouchedTaskID}"]`)).toBeVisible();

	let saveRequestCount = 0;
	page.on('request', (request) => {
		if (request.url().includes('/rest/v1/rpc/task_save')) saveRequestCount += 1;
	});

	await columnDropZone(page, 'in_progress').evaluate((element, taskID) => {
		const dataTransfer = new DataTransfer();
		dataTransfer.setData('text/plain', taskID);
		element.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }));
		element.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }));
	}, untouchedTaskID);

	await expect(taskColumn(page, 'in_progress').locator(`[data-task-board-card="${untouchedTaskID}"]`)).toHaveCount(0);
	expect(saveRequestCount).toBe(0);
	expect(await taskStatusOf(untouchedTaskID)).toBe('planned');
});

test('the column add button writes a task the record gives that column', async ({ page }) => {
	const title = 'E2E 보드 진행 칼럼에서 추가한 업무';
	await signInToTheTaskBoard(page);

	await taskColumn(page, 'in_progress').getByRole('button', { name: '진행 업무 추가' }).first().click();
	const sheet = taskSheet(page);
	await expect(sheet).toBeVisible();
	await sheet.getByPlaceholder('업무 내용').fill(title);
	await sheet.getByRole('button', { name: '업무 저장' }).click();
	await expect(sheet).not.toBeVisible();

	const written = await taskRowTitled(title);
	expect(written).not.toBeNull();
	if (!written) return;
	seededTaskIDs.push(written.id);
	expect(written.status).toBe('in_progress');
	expect(await taskParticipantIDsOf(written.id)).toEqual([member1ID]);
	await expect(taskColumn(page, 'in_progress').locator(`[data-task-board-card="${written.id}"]`)).toBeVisible();
});

test('a card the signed-in member may not change cannot be dragged', async ({ page }) => {
	const [othersTaskID] = await seedForThisRun([
		{ title: 'E2E 보드 남의 업무', status: 'planned', participantIDs: [member2ID] }
	]);
	await signInToTheTaskBoard(page, member3Email);
	await showEveryParticipant(page);

	const card = taskCard(page, othersTaskID);
	await expect(card).toBeVisible();
	await expect(card).toHaveAttribute('draggable', 'false');
});

test('a card the signed-in member participates in can be dragged', async ({ page }) => {
	const [ownTaskID] = await seedForThisRun([
		{ title: 'E2E 보드 내 업무', status: 'planned', participantIDs: [member3ID] }
	]);
	await signInToTheTaskBoard(page, member3Email);

	const card = taskCard(page, ownTaskID);
	await expect(card).toBeVisible();
	await expect(card).toHaveAttribute('draggable', 'true');
});

test('the board scrolls sideways on a wide screen rather than widening the page', async ({ page }) => {
	await page.setViewportSize({ width: 700, height: 844 });
	await signInToTheTaskBoard(page);

	const scroller = page.locator('[data-task-board-scroll]');
	const measurements = await scroller.evaluate((element) => ({
		clientWidth: element.clientWidth,
		scrollWidth: element.scrollWidth,
		scrollLeft: element.scrollLeft
	}));
	expect(measurements.clientWidth).toBeGreaterThan(0);
	expect(measurements.scrollWidth).toBeGreaterThan(measurements.clientWidth);
	expect(measurements.scrollLeft).toBe(0);
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('the board stacks its columns on a phone rather than scrolling sideways', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheTaskBoard(page);

	const scroller = page.locator('[data-task-board-scroll]');
	const measurements = await scroller.evaluate((element) => ({
		clientWidth: element.clientWidth,
		scrollWidth: element.scrollWidth
	}));
	expect(measurements.scrollWidth).toBeLessThanOrEqual(measurements.clientWidth);
	const planned = await taskColumn(page, 'planned').boundingBox();
	const inProgress = await taskColumn(page, 'in_progress').boundingBox();
	expect(planned).not.toBeNull();
	expect(inProgress).not.toBeNull();
	if (!planned || !inProgress) return;
	expect(inProgress.x).toBe(planned.x);
	expect(inProgress.y).toBeGreaterThanOrEqual(planned.y + planned.height);
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

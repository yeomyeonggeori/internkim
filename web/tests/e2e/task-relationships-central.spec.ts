import { expect, test } from '@playwright/test';
import { member1ID } from './central-test-utils';
import {
	openTaskCard,
	removeTasks,
	seedTasks,
	signInToTheTaskBoard,
	taskCard,
	taskRowOf,
	taskSheet,
	type CentralTaskStatus
} from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const parentTitle = 'E2E 관계 부모 업무';
const looseTitle = 'E2E 관계 아직 연결되지 않은 업무';

let seededTaskIDs: string[] = [];
let parentTaskID = '';
let completedChildTaskID = '';
let openChildTaskID = '';
let looseTaskID = '';

async function seedTask(title: string, status: CentralTaskStatus, parentID?: string): Promise<string> {
	const [identifier] = await seedTasks([
		{
			title,
			status,
			participantIDs: [member1ID],
			business: '사업하나',
			type: '기능',
			size: 'M',
			...(parentID ? { parentTaskID: parentID } : {})
		}
	]);
	seededTaskIDs.push(identifier);
	return identifier;
}

test.beforeAll(async () => {
	parentTaskID = await seedTask(parentTitle, 'in_progress');
	completedChildTaskID = await seedTask('E2E 관계 끝난 자녀 업무', 'completed', parentTaskID);
	openChildTaskID = await seedTask('E2E 관계 남은 자녀 업무', 'planned', parentTaskID);
	looseTaskID = await seedTask(looseTitle, 'planned');
});

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

test('the detail shows the children the record hangs under the task', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await openTaskCard(page, parentTaskID);

	const sheet = taskSheet(page);
	await expect(sheet.getByRole('heading', { name: '업무 관계' })).toBeVisible();
	await expect(sheet.locator(`[data-task-relationship-task="${completedChildTaskID}"]`)).toBeVisible();
	await expect(sheet.locator(`[data-task-relationship-task="${openChildTaskID}"]`)).toBeVisible();
	await expect(sheet.locator(`[data-task-relationship-task="${looseTaskID}"]`)).toHaveCount(0);
});

test('the detail shows the parent the record gives a child', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await openTaskCard(page, openChildTaskID);

	const sheet = taskSheet(page);
	const parentRow = sheet.locator(`[data-task-relationship-task="${parentTaskID}"]`);
	await expect(parentRow).toBeVisible();
	await expect(parentRow).toContainText(parentTitle);
});

test('the board card counts the children the record has completed', async ({ page }) => {
	await signInToTheTaskBoard(page);

	const progress = taskCard(page, parentTaskID).getByRole('progressbar');
	await expect(progress).toHaveAttribute('aria-valuenow', '1');
	await expect(progress).toHaveAttribute('aria-valuemax', '2');
	await expect(taskCard(page, parentTaskID).locator('[data-task-child-progress]')).toContainText('1/2');
});

test('a child connected in the sheet is a child on the record', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await openTaskCard(page, parentTaskID);

	const sheet = taskSheet(page);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	await sheet.getByRole('button', { name: '자녀 업무 추가' }).click();

	const selector = page.getByRole('dialog', { name: '자녀 업무' });
	await expect(selector).toBeVisible();
	await selector.getByPlaceholder('자녀 업무 검색').fill(looseTitle);
	await selector.getByRole('option', { name: new RegExp(looseTitle) }).click();
	await selector.getByRole('button', { name: '선택한 업무 연결' }).click();
	await expect(selector).not.toBeVisible();

	expect((await taskRowOf(looseTaskID))?.parent_task_id).toBe(parentTaskID);
	await expect(sheet.locator(`[data-task-relationship-task="${looseTaskID}"]`)).toBeVisible();
});

test('a relationship released in the sheet is released on the record', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await openTaskCard(page, parentTaskID);

	const sheet = taskSheet(page);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	const childRow = sheet.locator(`[data-task-relationship-task="${looseTaskID}"]`);
	await expect(childRow).toBeVisible();
	await childRow.getByRole('button', { name: '관계 작업 더보기' }).click();
	await page.getByRole('menuitem', { name: '관계 해제' }).click();

	await expect(sheet.locator(`[data-task-relationship-task="${looseTaskID}"]`)).toHaveCount(0);
	expect((await taskRowOf(looseTaskID))?.parent_task_id).toBeNull();
});

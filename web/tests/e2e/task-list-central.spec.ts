import { expect, test, type Page } from '@playwright/test';
import { member1ID, member2ID } from './central-test-utils';
import {
	expectTaskStatus,
	openTaskFilters,
	removeTasks,
	seedTasks,
	signInToTheTaskBoard
} from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const plainTaskTitle = 'E2E 목록 보통 업무';
const requestedTaskTitle = 'E2E 목록 요청받은 업무';
const everyStatusLabels = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];
const statusLabelsWithoutTheRequestLifecycle = ['예정', '진행', '완료', '일시정지', '중단'];

let seededTaskIDs: string[] = [];
let plainTaskID = '';

test.beforeAll(async () => {
	seededTaskIDs = await seedTasks([
		{
			title: plainTaskTitle,
			status: 'planned',
			participantIDs: [member1ID],
			business: '사업하나',
			type: '기능',
			size: 'M'
		},
		{
			title: requestedTaskTitle,
			status: 'requested',
			participantIDs: [member1ID],
			requesterID: member2ID,
			business: '사업하나',
			type: '기능',
			size: 'M'
		}
	]);
	[plainTaskID] = seededTaskIDs;
});

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

async function openTheListOfSeededTasks(page: Page): Promise<void> {
	await signInToTheTaskBoard(page);
	const panel = await openTaskFilters(page);
	await panel.getByPlaceholder('내용, 목표, 참여자 검색').fill('E2E 목록');
	await page.keyboard.press('Escape');
	await page.getByRole('tab', { name: '표', exact: true }).click();
	await expect(page.getByRole('row', { name: new RegExp(plainTaskTitle) })).toBeVisible();
}

function statusCellOf(page: Page, title: string) {
	return page.getByRole('row', { name: new RegExp(title) }).locator('[data-slot="select-trigger"]');
}

test('a status chosen in the list is the status the record holds', async ({ page }) => {
	await openTheListOfSeededTasks(page);

	await statusCellOf(page, plainTaskTitle).click();
	await page.getByRole('option', { name: '진행', exact: true }).click();

	await expectTaskStatus(plainTaskID, 'in_progress');
	await expect(statusCellOf(page, plainTaskTitle)).toHaveText('진행');
});

test('the list offers every status only for the task the record gives a requester', async ({ page }) => {
	await openTheListOfSeededTasks(page);

	await statusCellOf(page, requestedTaskTitle).click();
	expect(await page.getByRole('option').allTextContents()).toEqual(everyStatusLabels);
	await page.keyboard.press('Escape');

	await statusCellOf(page, plainTaskTitle).click();
	expect(await page.getByRole('option').allTextContents()).toEqual(statusLabelsWithoutTheRequestLifecycle);
});

import { expect, test } from '@playwright/test';
import { member1ID, member2ID } from './central-test-utils';
import {
	chooseTaskFilter,
	openTaskFilters,
	removeTasks,
	seedTasks,
	showEveryParticipant,
	signInToTheTaskBoard,
	taskCard
} from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

let seededTaskIDs: string[] = [];
let ownFirstBusinessTaskID = '';
let ownSecondBusinessTaskID = '';
let ownUntypedTaskID = '';
let anotherMembersTaskID = '';

test.beforeAll(async () => {
	seededTaskIDs = await seedTasks([
		{
			title: 'E2E 필터 사업하나 업무',
			status: 'planned',
			participantIDs: [member1ID],
			business: '사업하나',
			type: '기능',
			size: 'M'
		},
		{
			title: 'E2E 필터 사업둘 업무',
			status: 'planned',
			participantIDs: [member1ID],
			business: '사업둘',
			type: '회의',
			size: 'M'
		},
		{
			title: 'E2E 필터 종류 없는 업무',
			status: 'planned',
			participantIDs: [member1ID],
			business: '사업하나',
			type: null,
			size: 'M'
		},
		{
			title: 'E2E 필터 다른 구성원 업무',
			status: 'planned',
			participantIDs: [member2ID],
			business: '사업하나',
			type: '기능',
			size: 'M'
		}
	]);
	[ownFirstBusinessTaskID, ownSecondBusinessTaskID, ownUntypedTaskID, anotherMembersTaskID] = seededTaskIDs;
});

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

test('the board opens on the signed-in member and 전체 brings the rest back', async ({ page }) => {
	await signInToTheTaskBoard(page);

	await expect(page.getByRole('button', { name: '필터 1' })).toBeVisible();
	await expect(taskCard(page, ownFirstBusinessTaskID)).toBeVisible();
	await expect(taskCard(page, anotherMembersTaskID)).toHaveCount(0);

	await showEveryParticipant(page);

	await expect(page.getByRole('button', { name: '필터', exact: true })).toBeVisible();
	await expect(taskCard(page, anotherMembersTaskID)).toBeVisible();
	await expect(taskCard(page, ownFirstBusinessTaskID)).toBeVisible();
});

test('the business filter keeps only the tasks the record gives that business', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await chooseTaskFilter(page, '사업', '사업둘');

	await expect(taskCard(page, ownSecondBusinessTaskID)).toBeVisible();
	await expect(taskCard(page, ownFirstBusinessTaskID)).toHaveCount(0);
	await expect(taskCard(page, ownUntypedTaskID)).toHaveCount(0);
});

test('the type filter keeps only the tasks the record gives that type', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await chooseTaskFilter(page, '종류', '회의');

	await expect(taskCard(page, ownSecondBusinessTaskID)).toBeVisible();
	await expect(taskCard(page, ownFirstBusinessTaskID)).toHaveCount(0);
});

test('기타 keeps only the tasks the record gives no type', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await chooseTaskFilter(page, '종류', '기타');

	await expect(taskCard(page, ownUntypedTaskID)).toBeVisible();
	await expect(taskCard(page, ownFirstBusinessTaskID)).toHaveCount(0);
	await expect(taskCard(page, ownSecondBusinessTaskID)).toHaveCount(0);
});

test('the search box narrows the board to the matching task', async ({ page }) => {
	await signInToTheTaskBoard(page);
	const panel = await openTaskFilters(page);
	await panel.getByPlaceholder('내용, 목표, 참여자 검색').fill('사업둘 업무');
	await page.keyboard.press('Escape');

	await expect(taskCard(page, ownSecondBusinessTaskID)).toBeVisible();
	await expect(taskCard(page, ownFirstBusinessTaskID)).toHaveCount(0);
});

test('초기화 puts the participant filter back on the signed-in member', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await showEveryParticipant(page);
	await expect(taskCard(page, anotherMembersTaskID)).toBeVisible();

	const panel = await openTaskFilters(page);
	await panel.getByRole('button', { name: '초기화' }).click();
	await page.keyboard.press('Escape');

	await expect(page.getByRole('button', { name: '필터 1' })).toBeVisible();
	await expect(taskCard(page, anotherMembersTaskID)).toHaveCount(0);
	await expect(taskCard(page, ownFirstBusinessTaskID)).toBeVisible();
});

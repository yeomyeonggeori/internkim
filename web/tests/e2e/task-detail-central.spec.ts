import { expect, test } from '@playwright/test';
import {
	member1ID,
	member1Name,
	member2ID,
	member2Name,
	member3Email,
	member3ID,
	member3Name
} from './central-test-utils';
import {
	chooseEditorOption,
	openEditorSelectOptions,
	openTaskCard,
	removeTasks,
	seedTasks,
	showEveryParticipant,
	signInToTheTaskBoard,
	taskParticipantIDsOf,
	taskRowOf,
	taskSheet,
	weekEndInstant,
	weekStartDay,
	weekStartInstant,
	type CentralTaskSeed
} from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const everyStatusLabels = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];
const statusLabelsWithoutTheRequestLifecycle = ['예정', '진행', '완료', '일시정지', '중단'];

let seededTaskIDs: string[] = [];

async function seedOne(task: Partial<CentralTaskSeed> & { title: string }): Promise<string> {
	const [identifier] = await seedTasks([
		{
			status: 'planned',
			participantIDs: [member1ID],
			business: '사업둘',
			type: '개선',
			size: 'L',
			...task
		}
	]);
	seededTaskIDs.push(identifier);
	return identifier;
}

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

test('the detail sheet shows what the record holds for the task', async ({ page }) => {
	const taskID = await seedOne({
		title: 'E2E 상세 읽기 업무',
		participantIDs: [member1ID, member3ID],
		startsAtISO: weekStartInstant(),
		endsAtISO: weekEndInstant()
	});
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await expect(sheet.getByText('업무 상세')).toBeVisible();
	await expect(sheet.getByText('E2E 상세 읽기 업무')).toBeVisible();
	await expect(sheet.getByText('사업둘', { exact: true })).toBeVisible();
	await expect(sheet.getByText('개선', { exact: true })).toBeVisible();
	await expect(sheet.getByText('L', { exact: true })).toBeVisible();
	await expect(sheet.getByText(member1Name, { exact: true })).toBeVisible();
	await expect(sheet.getByText(member3Name, { exact: true })).toBeVisible();
	await expect(sheet.getByText(weekStartDay(), { exact: false })).toBeVisible();
});

test('an edit saved from the sheet is on the record', async ({ page }) => {
	const taskID = await seedOne({ title: 'E2E 상세 수정 전 업무' });
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	await sheet.getByPlaceholder('업무 내용').fill('E2E 상세 수정 후 업무');
	await chooseEditorOption(page, sheet, '종류', '회의');
	await chooseEditorOption(page, sheet, '상태', '진행');
	await sheet.getByRole('button', { name: '업무 저장' }).click();
	await expect(sheet).not.toBeVisible();

	const saved = await taskRowOf(taskID);
	expect(saved?.title).toBe('E2E 상세 수정 후 업무');
	expect(saved?.type).toBe('회의');
	expect(saved?.status).toBe('in_progress');
	expect(saved?.business).toBe('사업둘');
});

test('a task the record gives a requester shows that requester and every status', async ({ page }) => {
	const taskID = await seedOne({
		title: 'E2E 상세 요청받은 업무',
		status: 'requested',
		requesterID: member2ID
	});
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await expect(sheet.getByText('요청자')).toBeVisible();
	await expect(sheet.getByText(member2Name, { exact: true })).toBeVisible();

	await sheet.getByRole('button', { name: '업무 수정' }).click();
	expect(await openEditorSelectOptions(page, sheet, '상태')).toEqual(everyStatusLabels);
});

test('a task the record gives no requester offers only the statuses a plain task has', async ({ page }) => {
	const taskID = await seedOne({ title: 'E2E 상세 요청 없는 업무' });
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await expect(sheet.getByText('요청자')).toHaveCount(0);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	expect(await openEditorSelectOptions(page, sheet, '상태')).toEqual(statusLabelsWithoutTheRequestLifecycle);
});

test('a member who neither participates nor administers is told the sheet is read-only', async ({ page }) => {
	const taskID = await seedOne({ title: 'E2E 상세 남의 업무', participantIDs: [member1ID] });
	await signInToTheTaskBoard(page, member3Email);
	await showEveryParticipant(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await expect(sheet.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toBeVisible();
	await expect(sheet.getByRole('button', { name: '업무 수정' })).toHaveCount(0);
	await expect(sheet.getByRole('button', { name: '업무 저장' })).toHaveCount(0);
	await expect(sheet.getByRole('button', { name: '업무 삭제' })).toHaveCount(0);
});

test('a participant added in the sheet is on the record', async ({ page }) => {
	const taskID = await seedOne({ title: 'E2E 상세 참여자 추가 업무' });
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	await sheet.getByRole('combobox', { name: '참여자' }).click();
	await page.getByRole('option', { name: new RegExp(member3Name) }).click();
	await page.keyboard.press('Escape');
	await sheet.getByRole('button', { name: '업무 저장' }).click();
	await expect(sheet).not.toBeVisible();

	expect(await taskParticipantIDsOf(taskID)).toEqual([member1ID, member3ID].toSorted());
});

test('a task deleted from the sheet is gone from the record', async ({ page }) => {
	const taskID = await seedOne({ title: 'E2E 상세 삭제할 업무' });
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);

	const sheet = taskSheet(page);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	await sheet.getByRole('button', { name: '업무 삭제' }).click();
	const confirmation = page.getByRole('alertdialog').last();
	await expect(confirmation).toBeVisible();
	await confirmation.getByRole('button', { name: '삭제', exact: true }).click();
	await expect(sheet).not.toBeVisible();

	expect(await taskRowOf(taskID)).toBeNull();
});

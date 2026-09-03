import { expect, type Locator, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { centralPlaneAdminClient, exampleCompanyID } from './central-test-utils';

export type CentralTaskStatus =
	| 'requested'
	| 'planned'
	| 'in_progress'
	| 'completed'
	| 'paused'
	| 'rejected'
	| 'stopped';

export type CentralTaskSeed = {
	title: string;
	status: CentralTaskStatus;
	participantIDs: string[];
	business?: string | null;
	type?: string | null;
	size?: string | null;
	startsAtISO?: string | null;
	endsAtISO?: string | null;
	requesterID?: string;
	parentTaskID?: string;
};

export type CentralTaskRow = {
	id: string;
	title: string;
	status: string;
	business: string | null;
	type: string | null;
	size: string | null;
	starts_at: string | null;
	ends_at: string | null;
	parent_task_id: string | null;
	requester_id: string | null;
};

export type CentralTaskVocabularyEntry = { name: string; color?: string };

export type CentralTaskVocabulary = {
	businesses?: CentralTaskVocabularyEntry[];
	types?: CentralTaskVocabularyEntry[];
	etcBusinessColor?: string;
	etcTypeColor?: string;
};

const taskRowSelection =
	'id, title, status, business, type, size, starts_at, ends_at, parent_task_id, requester_id';
const seoulOffset = '+09:00';
const millisecondsPerDay = 24 * 60 * 60 * 1000;

export function weekStartDay(): string {
	return dayISOOf(mondayOf(new Date()));
}

function weekEndDay(): string {
	return dayISOOf(addDays(mondayOf(new Date()), 6));
}

export function weekStartInstant(): string {
	return `${weekStartDay()}T12:00:00${seoulOffset}`;
}

export function weekEndInstant(): string {
	return `${weekEndDay()}T12:00:00${seoulOffset}`;
}

export async function signInToTheTaskBoard(page: Page, email?: string): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/task', email);
	await page.getByRole('tab', { name: '보드', exact: true }).waitFor({ state: 'visible', timeout: 30000 });
	await expect(taskColumn(page, 'in_progress')).toBeVisible();
}

export async function seedTasks(tasks: CentralTaskSeed[]): Promise<string[]> {
	const admin = centralPlaneAdminClient();
	const identifiers: string[] = [];
	for (const task of tasks) {
		const inserted = await admin
			.from('task')
			.insert({
				company_id: exampleCompanyID,
				title: task.title,
				status: task.status,
				business: task.business ?? null,
				type: task.type ?? null,
				size: task.size ?? null,
				starts_at: task.startsAtISO ?? null,
				ends_at: task.endsAtISO ?? null,
				requester_id: task.requesterID ?? null,
				parent_task_id: task.parentTaskID ?? null
			})
			.select('id')
			.single<{ id: string }>();
		if (inserted.error) {
			await removeTasks(identifiers);
			throw new Error(`Failed to seed a task: ${inserted.error.message}`);
		}
		identifiers.push(inserted.data.id);
		if (task.participantIDs.length === 0) continue;
		const participants = await admin.from('task_participant').insert(
			task.participantIDs.map((memberID) => ({ task_id: inserted.data.id, member_id: memberID }))
		);
		if (participants.error) {
			await removeTasks(identifiers);
			throw new Error(`Failed to seed task participants: ${participants.error.message}`);
		}
	}
	return identifiers;
}

export async function removeTasks(taskIDs: string[]): Promise<void> {
	if (taskIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('task').delete().in('id', taskIDs);
	if (deleted.error) throw new Error(`Failed to clean up tasks: ${deleted.error.message}`);
}

export async function taskRowOf(taskID: string): Promise<CentralTaskRow | null> {
	const admin = centralPlaneAdminClient();
	const rows = await admin
		.from('task')
		.select(taskRowSelection)
		.eq('id', taskID)
		.returns<CentralTaskRow[]>();
	if (rows.error) throw new Error(`Failed to read a task: ${rows.error.message}`);
	return rows.data[0] ?? null;
}

export async function taskRowTitled(title: string): Promise<CentralTaskRow | null> {
	const admin = centralPlaneAdminClient();
	const rows = await admin
		.from('task')
		.select(taskRowSelection)
		.eq('company_id', exampleCompanyID)
		.eq('title', title)
		.returns<CentralTaskRow[]>();
	if (rows.error) throw new Error(`Failed to read a task by title: ${rows.error.message}`);
	return rows.data[0] ?? null;
}

export async function taskParticipantIDsOf(taskID: string): Promise<string[]> {
	const admin = centralPlaneAdminClient();
	const rows = await admin
		.from('task_participant')
		.select('member_id')
		.eq('task_id', taskID)
		.returns<{ member_id: string }[]>();
	if (rows.error) throw new Error(`Failed to read task participants: ${rows.error.message}`);
	return rows.data.map((row) => row.member_id).toSorted();
}

export async function taskStatusOf(taskID: string): Promise<string> {
	const row = await taskRowOf(taskID);
	if (!row) throw new Error(`task ${taskID} is no longer on the record`);
	return row.status;
}

export async function expectTaskStatus(taskID: string, status: CentralTaskStatus): Promise<void> {
	await expect
		.poll(() => taskStatusOf(taskID), { timeout: 15000 })
		.toBe(status);
}

export async function companyTaskVocabulary(): Promise<CentralTaskVocabulary> {
	const admin = centralPlaneAdminClient();
	const company = await admin
		.from('company')
		.select('task_vocabulary')
		.eq('id', exampleCompanyID)
		.single<{ task_vocabulary: CentralTaskVocabulary }>();
	if (company.error) throw new Error(`Failed to read the task vocabulary: ${company.error.message}`);
	return company.data.task_vocabulary;
}

export async function setCompanyTaskVocabulary(vocabulary: CentralTaskVocabulary): Promise<void> {
	const admin = centralPlaneAdminClient();
	const updated = await admin
		.from('company')
		.update({ task_vocabulary: vocabulary })
		.eq('id', exampleCompanyID);
	if (updated.error) throw new Error(`Failed to write the task vocabulary: ${updated.error.message}`);
}

export function taskColumn(page: Page, status: string): Locator {
	return page.locator(`[data-task-board-column="${status}"]`);
}

export function taskCard(page: Page, taskID: string): Locator {
	return page.locator(`[data-task-board-card="${taskID}"]`);
}

export function columnDropZone(page: Page, status: string): Locator {
	return page.locator(`[data-task-board-drop-zone="${status}"]`);
}

export function columnTaskCount(page: Page, status: string): Locator {
	return taskColumn(page, status).locator('[data-task-board-task-count]');
}

export function taskSheet(page: Page): Locator {
	return page.locator('[data-slot="sheet-content"]');
}

export async function openTaskCard(page: Page, taskID: string): Promise<void> {
	await taskCard(page, taskID).click();
	await expect(taskSheet(page)).toBeVisible();
}

export function editorSelect(sheet: Locator, fieldLabel: string): Locator {
	return sheet.locator('label').filter({ hasText: fieldLabel }).locator('[data-slot="select-trigger"]');
}

export async function openEditorSelectOptions(page: Page, sheet: Locator, fieldLabel: string): Promise<string[]> {
	await editorSelect(sheet, fieldLabel).click();
	const options = page.getByRole('option');
	await expect(options.first()).toBeVisible();
	return options.allTextContents();
}

export async function chooseEditorOption(page: Page, sheet: Locator, fieldLabel: string, optionLabel: string): Promise<void> {
	await editorSelect(sheet, fieldLabel).click();
	await page.getByRole('option', { name: optionLabel, exact: true }).click();
	await expect(page.getByRole('option')).toHaveCount(0);
}

export async function dragCardOntoColumn(page: Page, taskID: string, status: string): Promise<void> {
	await dragBetween(taskCard(page, taskID), columnDropZone(page, status));
}

async function dragBetween(source: Locator, target: Locator): Promise<void> {
	const bounds = await target.boundingBox();
	if (!bounds) throw new Error('the drop target has no bounding box');
	const dataTransfer = await source.evaluateHandle(() => new DataTransfer());
	await source.dispatchEvent('dragstart', { dataTransfer });
	await target.dispatchEvent('dragover', {
		clientY: bounds.y + Math.max(2, bounds.height / 2),
		dataTransfer
	});
	await target.dispatchEvent('drop', {
		clientY: bounds.y + Math.max(2, bounds.height / 2),
		dataTransfer
	});
	await source.dispatchEvent('dragend', { dataTransfer });
}

export async function openTaskFilters(page: Page): Promise<Locator> {
	await page.getByRole('button', { name: /^필터/ }).click();
	const panel = page.locator('[data-task-filter-panel]');
	await expect(panel).toBeVisible();
	return panel;
}

export async function chooseTaskFilter(page: Page, filterLabel: string, optionLabel: string): Promise<void> {
	const panel = await openTaskFilters(page);
	await panel.getByRole('combobox', { name: filterLabel, exact: true }).click();
	await page.getByRole('option', { name: optionLabel, exact: true }).click();
	await page.keyboard.press('Escape');
	await expect(panel).not.toBeVisible();
}

export async function showEveryParticipant(page: Page): Promise<void> {
	await chooseTaskFilter(page, '참여자', '전체');
}

function mondayOf(date: Date): Date {
	const midnight = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
	return addDays(midnight, -((midnight.getUTCDay() + 6) % 7));
}

function addDays(date: Date, days: number): Date {
	return new Date(date.getTime() + days * millisecondsPerDay);
}

function dayISOOf(date: Date): string {
	return date.toISOString().slice(0, 10);
}

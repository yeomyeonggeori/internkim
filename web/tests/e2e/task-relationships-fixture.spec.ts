import { expect, type Page, type Route, test } from '@playwright/test';
import type { RecordTask, RecordTaskList } from '../../src/lib/task/task-record';
import { taskWeekOfDate } from '../../src/lib/task/task-week-code';
import { openTaskCard, taskSheet } from './task-central-test-utils';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const targetTaskID = '30000000-0000-4000-8000-000000000001';
const parentTaskID = '30000000-0000-4000-8000-000000000002';
const childTaskID = '30000000-0000-4000-8000-000000000003';
const targetTitle = 'Fixture 관계를 확인하는 업무';
const parentTitle = 'Fixture 부모 후보 업무';
const childTitle = 'Fixture 연결할 자녀 업무';

type TaskWrite = {
	taskHint: string;
	parentTaskHint?: string;
	childTaskHints?: string[];
	title?: string;
};

function taskFixture(taskID: string, content: string): RecordTask {
	return {
		taskID, content, status: 'planned', size: 'M', ownerID: memberID,
		ownerName: '이샘플', participantIDs: [memberID], participantNames: ['이샘플'],
		createdAt: new Date().toISOString(), weekCode: taskWeekOfDate(new Date()).code
	};
}

class TaskRelationshipFixture {
	tasks = [taskFixture(targetTaskID, targetTitle), taskFixture(parentTaskID, parentTitle), taskFixture(childTaskID, childTitle)];
	historyReads = 0;
	failedHistoryReads = new Set<number>();
	writeGate = Promise.resolve();
	onWriteStarted = () => {};

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
		if (toolName === 'task_list') {
			this.historyReads++;
			if (this.failedHistoryReads.has(this.historyReads)) {
				await route.fulfill({ status: 503, json: { error: 'Temporarily unavailable' } });
				return;
			}
		}
		if (toolName === 'task_list' || toolName === 'task_board_get') {
			await route.fulfill({ json: { result: this.listed() } });
			return;
		}
		if (toolName === 'task_update') {
			await this.write(route);
			return;
		}
		await route.fulfill({ status: 503, json: { error: 'Tool has no supplementary UI fixture' } });
	}

	async write(route: Route): Promise<void> {
		const written: { input: TaskWrite } = route.request().postDataJSON();
		this.onWriteStarted();
		await this.writeGate;
		const task = this.tasks.find(task => task.taskID === written.input.taskHint);
		if (!task) throw new Error('Fixture write must target a seeded task');
		if (written.input.title !== undefined) task.content = written.input.title;
		if (written.input.parentTaskHint !== undefined) task.parentTaskID = written.input.parentTaskHint || undefined;
		for (const childID of written.input.childTaskHints ?? []) {
			const child = this.tasks.find(task => task.taskID === childID);
			if (!child) throw new Error('Fixture relationship must target a seeded child');
			child.parentTaskID = task.taskID;
		}
		await route.fulfill({ json: { result: task } });
	}
}

async function installAuthenticatedFixture(page: Page, fixture: TaskRelationshipFixture): Promise<void> {
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
		const pathname = new URL(route.request().url()).pathname;
		if (pathname === '/rest/v1/member') {
			await route.fulfill({ json: { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } });
			return;
		}
		await route.fulfill({ json: [] });
	});
	await page.route('**/api/member/me', route => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/agent/api/**', route => route.fulfill({ json: {} }));
	await page.route('**/api/v1/tools/*/invoke', route => fixture.answer(route));
	await page.goto('/example-co/task');
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await openTaskCard(page, targetTaskID);
	await taskSheet(page).getByRole('button', { name: '업무 수정' }).click();
}

test.use({ locale: 'ko-KR' });

test('supplementary fixture: ordinary editing demands no history until relationships open', async ({ page }) => {
	const fixture = new TaskRelationshipFixture();
	await installAuthenticatedFixture(page, fixture);
	const sheet = taskSheet(page);
	await sheet.getByPlaceholder('업무 내용').fill('Fixture 보통 수정한 업무');
	await sheet.getByRole('button', { name: '업무 저장', exact: true }).click();
	await expect(sheet).not.toBeVisible();
	expect(fixture.historyReads).toBe(0);
	expect(fixture.tasks.find(task => task.taskID === targetTaskID)?.content).toBe('Fixture 보통 수정한 업무');
	await openTaskCard(page, targetTaskID);
	await sheet.getByRole('button', { name: '업무 수정' }).click();
	await sheet.getByRole('button', { name: '업무 관계', exact: true }).click();
	await expect(sheet.getByRole('heading', { name: '업무 관계', exact: true })).toBeVisible();
	expect(fixture.historyReads).toBe(1);
});

test('supplementary fixture: child selector stays visible and disabled until the write commits', async ({ page }) => {
	const fixture = new TaskRelationshipFixture();
	await installAuthenticatedFixture(page, fixture);
	const sheet = taskSheet(page);
	await sheet.getByRole('button', { name: '업무 관계', exact: true }).click();
	await sheet.getByRole('button', { name: '자녀 업무 추가' }).click();
	const selector = page.getByRole('dialog', { name: '자녀 업무', exact: true });
	await selector.getByPlaceholder('자녀 업무 검색').fill(childTitle);
	await selector.getByRole('option', { name: new RegExp(childTitle) }).click();
	let releaseWrite = () => {};
	fixture.writeGate = new Promise<void>(resolve => { releaseWrite = resolve; });
	const writeStarted = new Promise<void>(resolve => { fixture.onWriteStarted = resolve; });
	try {
		await selector.getByRole('button', { name: '선택한 업무 연결' }).click();
		await writeStarted;
		await expect(selector).toBeVisible();
		await expect(selector.getByRole('button', { name: '선택한 업무 연결' })).toBeDisabled();
		await expect(selector.getByPlaceholder('자녀 업무 검색')).toBeDisabled();
		expect(fixture.tasks.find(task => task.taskID === childTaskID)?.parentTaskID).toBeUndefined();
		releaseWrite();
		await expect(selector).not.toBeVisible();
		await expect(sheet.locator(`[data-task-relationship-task="${childTaskID}"]`)).toBeVisible();
		expect(fixture.tasks.find(task => task.taskID === childTaskID)?.parentTaskID).toBe(targetTaskID);
	} finally { releaseWrite(); }
});

test('supplementary fixture: initial and retained history failures both retry with the draft preserved', async ({ page }) => {
	const fixture = new TaskRelationshipFixture();
	fixture.failedHistoryReads = new Set([1, 3]);
	await installAuthenticatedFixture(page, fixture);
	const sheet = taskSheet(page);
	const draftTitle = 'Fixture 재시도 뒤에도 남는 수정';
	await sheet.getByPlaceholder('업무 내용').fill(draftTitle);
	await sheet.getByRole('button', { name: '업무 관계', exact: true }).click();
	await expect(sheet.getByRole('status')).toContainText('Temporarily unavailable');
	await expect(sheet.getByPlaceholder('업무 내용')).toHaveValue(draftTitle);
	await sheet.getByRole('button', { name: '다시 불러오기', exact: true }).click();
	await expect(sheet.getByRole('heading', { name: '업무 관계', exact: true })).toBeVisible();
	expect(fixture.historyReads).toBe(2);
	await sheet.getByRole('button', { name: '부모 업무 추가' }).click();
	const selector = page.getByRole('dialog', { name: '부모 업무', exact: true });
	await selector.getByPlaceholder('부모 업무 검색').fill(parentTitle);
	await selector.getByRole('option', { name: new RegExp(parentTitle) }).click();
	await expect(selector).not.toBeVisible();
	await expect(sheet.getByRole('status')).toContainText('Temporarily unavailable');
	await expect(sheet.getByRole('heading', { name: '업무 관계', exact: true })).toBeVisible();
	await expect(sheet.getByRole('button', { name: '자녀 업무 추가' })).toHaveCount(0);
	await expect(sheet.getByPlaceholder('업무 내용')).toHaveValue(draftTitle);
	await sheet.getByRole('button', { name: '다시 불러오기', exact: true }).click();
	await expect(sheet.getByRole('button', { name: '자녀 업무 추가' })).toBeEnabled();
	await expect(sheet.getByPlaceholder('업무 내용')).toHaveValue(draftTitle);
	await expect(sheet.locator(`[data-task-relationship-task="${parentTaskID}"]`)).toBeVisible();
	expect(fixture.historyReads).toBe(4);
	expect(fixture.tasks.find(task => task.taskID === targetTaskID)?.parentTaskID).toBe(parentTaskID);
});

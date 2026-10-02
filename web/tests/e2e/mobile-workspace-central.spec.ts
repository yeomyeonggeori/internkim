import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { member1ID, member3ID } from './central-test-utils';
import { openTaskFilters, seedTasks, removeTasks, taskRowOf } from './task-central-test-utils';
import { seedAttendanceEvents, cleanupAttendanceEvents, seoulInstant, seoulMonthToday, monthBefore, dayOfMonth } from './attendance-central-test-utils';
import { readFileSync } from 'node:fs';
import type { DataRoomCategory, DataRoomRole } from '../../src/lib/data-room/model';
const dataRoomTemplate: { categories: DataRoomCategory[]; roles: DataRoomRole[] } = JSON.parse(readFileSync(new URL('../../src/lib/data-room/template.json', import.meta.url), 'utf8'));

test.describe.configure({ mode: 'serial', timeout: 150_000 });
test.use({ locale: 'ko-KR' });
test.beforeEach(async ({ page }) => { page.setDefaultTimeout(10_000); });
const title = 'E2E 모바일 UX 검토용 긴 업무 제목과 상세 정보';
let taskIDs: string[] = [];
let attendanceIDs: string[] = [];
const writtenDate = dayOfMonth(monthBefore(seoulMonthToday()), 15);

test.beforeAll(async () => {
	taskIDs = await seedTasks([{ title, status: 'planned', participantIDs: [member1ID], size: 'M' }]);
	try {
		attendanceIDs = await seedAttendanceEvents([{ memberID: member3ID, kind: 'clock_in', occurredAtISO: seoulInstant(writtenDate, '10:00'), originalOccurredAtISO: seoulInstant(writtenDate, '09:00'), editReason: '모바일 UX 검토용 수정 기록' }]);
	} catch (error) { await removeTasks(taskIDs); taskIDs = []; throw error; }
});
test.afterAll(async () => { await cleanupAttendanceEvents(attendanceIDs); await removeTasks(taskIDs); });

async function fits(page: Page, name: string) {
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width);
	const output = process.env.MOBILE_UX_SCREENSHOTS;
	if (output) await page.screenshot({ path: `${output}/${name}-${page.viewportSize()!.width}.png` });
}

for (const width of [320, 360, 390, 568, 1280]) {
	test(`task, directory, attendance and document categories at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: width === 568 ? 320 : 844 });
		await signInToTheCentralPlane(page, '/example-co/task');
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		const filters = await openTaskFilters(page);
		await filters.getByPlaceholder('내용, 목표, 참여자 검색').fill(title);
		await page.keyboard.press('Escape');
		await page.getByRole('tab', { name: '목록', exact: true }).click();
		if (width < 640) {
			const task = page.getByRole('listitem').filter({ has: page.getByRole('button', { name: title, exact: true }) });
			await expect(task).toBeVisible();
			await task.getByRole('button', { name: '업무 정보', exact: true }).click();
			await expect(task).toContainText('M');
			await fits(page, 'task-list-details');
			await task.locator('[data-slot="select-trigger"]').click();
			await page.getByRole('option', { name: '진행', exact: true }).click();
			await expect.poll(async () => (await taskRowOf(taskIDs[0]))?.status).toBe('in_progress');
			await page.getByRole('button', { name: '정렬', exact: true }).click();
			await expect(page.getByRole('button', { name: /시작일/ })).toBeVisible();
			await task.getByRole('button', { name: title, exact: true }).click();
			await expect(page.getByRole('dialog')).toBeVisible();
			await fits(page, 'task-edit');
			await page.keyboard.press('Escape');
		}
		await page.getByRole('tab', { name: '정의', exact: true }).click();
		await fits(page, 'task-definitions');
		if (width < 640) { await page.locator('[data-slot="accordion-trigger"]').first().click(); await fits(page, 'task-size-details'); }
		await page.getByRole('tab', { name: '구성원', exact: true }).click();
		await fits(page, 'task-members');
		await page.goto('/example-co/organization');
		const person = page.getByTestId(`organization-person-node-${member1ID}`);
		await expect(person).toBeVisible();
		await fits(page, 'organization');
		await person.click();
		await expect(page.getByTestId('organization-person-detail-panel')).toBeVisible();
		await fits(page, 'organization-detail');
		if (width < 1024) await page.keyboard.press('Escape');
		await page.goto('/example-co/attendance');
		if (width < 768) await page.getByRole('tab', { name: '내 기록', exact: true }).click();
		await expect(page.getByTestId('personal-tools-panel').filter({ visible: true }).first()).toBeVisible();
		await fits(page, 'attendance');
		if (width < 640) {
			await page.getByRole('tab', { name: '직원별 휴가', exact: true }).click();
			const management = page.getByTestId('leave-management-view');
			await expect(management.getByRole('button', { name: /member1@example.com/ })).toBeVisible();
			await fits(page, 'leave-management');
			await management.getByRole('button', { name: /member1@example.com/ }).click();
			await expect(page.getByTestId('leave-management-employee-detail-header')).toBeVisible();
			await fits(page, 'leave-management-detail');
			await expect(page.getByRole('dialog').getByTestId('leave-management-refresh')).toBeVisible();
			await page.keyboard.press('Escape');
			if (width === 320) {
				await page.route('**/api/v1/tools/company_settings_get/invoke', route => route.fulfill({ status: 503, json: { message: 'test unavailable' } }));
				await management.getByRole('button', { name: /member3@example.com/ }).click();
				await expect(page.getByRole('dialog').getByRole('alert')).toBeVisible();
				await expect(page.getByTestId('leave-management-employee-detail-header')).toBeHidden();
				await page.unroute('**/api/v1/tools/company_settings_get/invoke');
				await page.getByRole('dialog').getByTestId('leave-management-refresh').click();
				await expect(page.getByTestId('leave-management-employee-detail-header')).toContainText('member3@example.com');
				await page.keyboard.press('Escape');
			}
			await page.getByRole('tab', { name: '손으로 쓴 기록', exact: true }).click();
			const record = page.locator(`[data-event-id="${attendanceIDs[0]}"]`);
			await expect(record).toBeVisible();
			await expect(record.getByTestId('hand-written-before')).toContainText('09:00');
			await fits(page, 'attendance-hand-written');
			await record.getByTestId('hand-written-undo').click();
			await expect(page.getByTestId('hand-written-undo-dialog')).toBeVisible();
			await fits(page, 'attendance-undo-confirm');
			await page.getByRole('button', { name: '취소', exact: true }).click();
		}
		await page.route('**/api/v1/tools/company_dataroom_get/invoke', route => route.fulfill({ json: { result: { categories: dataRoomTemplate.categories, roles: dataRoomTemplate.roles, shares: [], canManage: true } } }));
		await page.route('**/api/v1/tools/company_document_list/invoke', route => route.fulfill({ json: { result: { count: 0, documents: [] } } }));
		await page.goto('/example-co/data-room');
		if (width < 768) {
			await page.getByRole('button', { name: '문서 분류', exact: true }).click();
			await expect(page.getByRole('option')).toHaveCount(dataRoomTemplate.categories.length + 1);
			await page.getByRole('option').last().click();
			await expect(page.getByRole('listbox')).toBeHidden();
		}
		await fits(page, 'data-room');
		await page.getByRole('button', { name: '공유', exact: true }).click();
		await page.getByRole('button', { name: '커스텀 역할 만들기', exact: true }).click();
		await expect(page.getByRole('checkbox').first()).toBeVisible();
		await fits(page, 'data-room-role');
	});
}

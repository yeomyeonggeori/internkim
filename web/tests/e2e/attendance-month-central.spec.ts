import { expect, test, type Page } from '@playwright/test';
import {
	attendanceRowsOf,
	cleanupAttendanceEvents,
	dayOfMonth,
	monthBefore,
	removeAttendanceOf,
	seedAttendanceEvents,
	seoulInstant,
	seoulMonthToday,
	signInToAttendance
} from './attendance-central-test-utils';
import { member1Email, member1ID, member1Name, member3Email, member3ID, member3Name } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

const seededMonth = monthBefore(seoulMonthToday());
const seededDate = dayOfMonth(seededMonth, 15);
const office = '사무실';
const home = '재택';

let seededEventIDs: string[] = [];

test.beforeAll(async () => {
	await removeAttendanceOf(member1ID);
	await removeAttendanceOf(member3ID);
	seededEventIDs = await seedAttendanceEvents([
		{ memberID: member1ID, kind: 'clock_in', location: office, occurredAtISO: seoulInstant(seededDate, '09:00') },
		{ memberID: member1ID, kind: 'clock_out', occurredAtISO: seoulInstant(seededDate, '12:00') },
		{ memberID: member1ID, kind: 'clock_in', location: home, occurredAtISO: seoulInstant(seededDate, '13:00') },
		{ memberID: member1ID, kind: 'clock_out', occurredAtISO: seoulInstant(seededDate, '18:00') },
		{ memberID: member3ID, kind: 'clock_in', location: office, occurredAtISO: seoulInstant(seededDate, '10:00') },
		{ memberID: member3ID, kind: 'clock_out', occurredAtISO: seoulInstant(seededDate, '19:00') }
	]);
});

test.afterAll(async () => {
	await cleanupAttendanceEvents(seededEventIDs);
});

function monthLabel(month: string): string {
	const [year, monthNumber] = month.split('-').map(Number);
	return `${year}년 ${monthNumber}월`;
}

function statusTable(page: Page) {
	return page.getByTestId('team-status-table');
}

function seededCell(page: Page, email: string) {
	return page.getByTestId(`team-status-cell-${email}-${seededDate}`);
}

async function openSeededMonth(page: Page): Promise<void> {
	await signInToAttendance(page);
	await statusTable(page).waitFor({ state: 'visible', timeout: 30000 });
	await page.getByRole('button', { name: '이전 달' }).first().click();
	await expect(page.getByRole('button', { name: monthLabel(seededMonth) })).toBeVisible({
		timeout: 20000
	});
	await seededCell(page, member1Email).waitFor({ state: 'visible', timeout: 20000 });
}

test('the month table shows the people and the days the record holds', async ({ page }) => {
	await openSeededMonth(page);

	await expect(page.getByTestId(`team-status-person-header-${member1Email}`)).toContainText(member1Name);
	await expect(page.getByTestId(`team-status-person-header-${member3Email}`)).toContainText(member3Name);
	await expect(page.getByTestId(`team-status-day-${seededDate}`)).toBeVisible();
	await expect(seededCell(page, member3Email)).toBeVisible();
});

test('the month picker walks back to the current month', async ({ page }) => {
	await openSeededMonth(page);

	await page.getByRole('button', { name: '다음 달' }).first().click();

	await expect(page.getByRole('button', { name: monthLabel(seoulMonthToday()) })).toBeVisible({
		timeout: 20000
	});
	await expect(seededCell(page, member1Email)).toHaveCount(0);
});

test('a day cell tells its work segments on hover', async ({ page }) => {
	await openSeededMonth(page);

	await seededCell(page, member1Email).hover();

	const tooltip = page.locator('[data-slot="tooltip-content"]');
	await expect(tooltip).toBeVisible({ timeout: 20000 });
	await expect(tooltip.getByText(office)).toBeVisible();
	await expect(tooltip.getByText(home)).toBeVisible();
	await expect(tooltip.getByLabel('09:00-12:00')).toBeVisible();
	await expect(tooltip.getByLabel('13:00-18:00')).toBeVisible();
});

test('a day cell opens the day the record wrote', async ({ page }) => {
	await openSeededMonth(page);

	await seededCell(page, member1Email).click();

	const detail = page.getByTestId('team-status-day-detail-dialog');
	await expect(detail).toBeVisible({ timeout: 20000 });
	await expect(detail.getByTestId('team-status-day-detail-person')).toHaveText(member1Name);
	await expect(detail.getByTestId('team-status-work-record-header')).toContainText('2구간');
	await expect(detail.getByTestId('team-status-day-segment')).toHaveCount(2);
	await expect(detail.getByLabel('09:00-12:00')).toBeVisible();
	await expect(detail.getByLabel('13:00-18:00')).toBeVisible();
});

test('the person header opens that person work time', async ({ page }) => {
	await openSeededMonth(page);

	await page.getByTestId(`team-status-person-header-${member3Email}`).hover();

	const workTimeCard = page.locator('[data-slot="hover-card-content"]');
	await expect(workTimeCard).toBeVisible({ timeout: 20000 });
	await expect(workTimeCard.getByText('근무 시간', { exact: true })).toBeVisible();
});

test('an edited clock-out moves the row the record keeps', async ({ page }) => {
	await openSeededMonth(page);
	await seededCell(page, member1Email).click();

	const detail = page.getByTestId('team-status-day-detail-dialog');
	await detail.waitFor({ state: 'visible', timeout: 20000 });
	await detail.getByTestId('work-record-edit-button').click();

	const lastSegment = detail.getByTestId('team-status-day-segment').last();
	await lastSegment.getByLabel('퇴근').fill('17:00');
	await detail.getByTestId('work-record-edit-actions').getByRole('textbox').fill('E2E 근무 기록 정정');
	await detail.getByRole('button', { name: '저장', exact: true }).click();

	await expect
		.poll(async () => {
			const rows = await attendanceRowsOf(member1ID);
			return rows
				.filter((row) => row.kind === 'clock_out')
				.map((row) => new Date(row.occurred_at).getTime());
		}, { timeout: 30000 })
		.toContain(new Date(seoulInstant(seededDate, '17:00')).getTime());
	await expect(detail.getByLabel('13:00-17:00')).toBeVisible({ timeout: 20000 });
});

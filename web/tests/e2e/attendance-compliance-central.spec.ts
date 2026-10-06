import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import {
	cleanupAttendanceEvents,
	openMonthlyAttendance,
	dayOfMonth,
	monthBefore,
	removeAttendanceOf,
	removeCompanyWorkPolicy,
	seedAttendanceEvents,
	seoulInstant,
	seoulMonthToday
} from './attendance-central-test-utils';
import { member1Email, member1ID } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const wholeDayStart = '00:01';
const wholeDayEnd = '23:59';
const workedMonth = monthBefore(seoulMonthToday());
const workedDate = dayOfMonth(workedMonth, 15);

let seededEventIDs: string[] = [];

test.beforeAll(async () => {
	await removeAttendanceOf(member1ID);
	seededEventIDs = await seedAttendanceEvents([
		{
			memberID: member1ID,
			kind: 'clock_in',
			location: '사무실',
			occurredAtISO: seoulInstant(workedDate, '09:00')
		},
		{ memberID: member1ID, kind: 'clock_out', occurredAtISO: seoulInstant(workedDate, '18:00') }
	]);
});

test.beforeEach(async () => {
	await removeCompanyWorkPolicy();
});

test.afterAll(async () => {
	await cleanupAttendanceEvents(seededEventIDs);
	await removeCompanyWorkPolicy();
});

async function openSettings(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/settings');
	await page.getByRole('tab', { name: '관리자' }).click();
	await page
		.getByTestId('attendance-work-settings')
		.waitFor({ state: 'visible', timeout: 20000 });
}

async function chooseWorkMode(page: Page, mode: string): Promise<void> {
	const workSettings = page.getByTestId('attendance-work-settings');
	await workSettings.getByRole('button', { name: mode }).click();
}

async function saveWorkSettings(page: Page): Promise<void> {
	const workSettings = page.getByTestId('attendance-work-settings');
	await workSettings.getByRole('button', { name: '변경사항 저장' }).click();
	await expect(page.getByText('근무 설정을 저장했습니다.')).toBeVisible({ timeout: 20000 });
}

async function openTheMonthBefore(page: Page): Promise<void> {
	await page.goto('/example-co/attendance');
	await openMonthlyAttendance(page);
	await page.getByTestId('team-status-table').waitFor({ state: 'visible', timeout: 20000 });
	await page.getByRole('dialog').getByRole('button', { name: '이전 달', exact: true }).click();
	await page.getByTestId('team-status-table').waitFor({ state: 'visible', timeout: 20000 });
}

function complianceMarks(page: Page) {
	return page.locator('[data-testid^="team-status-compliance-"]');
}

function personalPanel(page: Page) {
	return page.getByTestId('personal-work-standard');
}

function workedDayCell(page: Page) {
	return page.getByTestId(`team-status-cell-${member1Email}-${workedDate}`);
}

test('a fixed schedule nobody meets is reported on both surfaces', async ({ page }) => {
	await openSettings(page);
	await chooseWorkMode(page, '고정 근무제');
	const workSettings = page.getByTestId('attendance-work-settings');
	await workSettings.getByLabel('출퇴근 시간 시작').fill(wholeDayStart);
	await workSettings.getByLabel('출퇴근 시간 종료').fill(wholeDayEnd);
	await saveWorkSettings(page);

	await openTheMonthBefore(page);

	await page.getByRole('dialog').getByRole('button', { name: '근무 기준', exact: true }).click();
	await expect(complianceMarks(page).first()).toBeVisible({ timeout: 20000 });
	await expect(personalPanel(page).getByTestId('work-standard-compliance-late')).toBeVisible({
		timeout: 20000
	});
	await expect(personalPanel(page).getByTestId('work-standard-compliance-earlyLeave')).toBeVisible({
		timeout: 20000
	});
});

test('an autonomous schedule reports none of the three', async ({ page }) => {
	await openSettings(page);
	await chooseWorkMode(page, '자율 근무제');
	await saveWorkSettings(page);

	await openTheMonthBefore(page);
	await page.getByRole('dialog').getByRole('button', { name: '근무 기준', exact: true }).click();
	await expect(personalPanel(page)).toContainText(`${dayOfMonth(workedMonth, 1)}–`, {
		timeout: 20000
	});

	await expect(personalPanel(page)).toContainText('자율 근무제');
	await expect(workedDayCell(page)).toHaveText(/\d/);
	await expect(complianceMarks(page)).toHaveCount(0);
	await expect(
		personalPanel(page).getByTestId('work-standard-compliance-late')
	).toHaveCount(0);
	await expect(
		personalPanel(page).getByTestId('work-standard-compliance-earlyLeave')
	).toHaveCount(0);
	await expect(
		personalPanel(page).getByTestId('work-standard-compliance-coreTimeMissed')
	).toHaveCount(0);
});

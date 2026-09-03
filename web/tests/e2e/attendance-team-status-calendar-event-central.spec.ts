import { expect, test, type Page } from '@playwright/test';
import { dayOfMonth, seoulInstant, seoulMonthToday, signInToAttendance } from './attendance-central-test-utils';
import { cleanupCalendarEvents, seedCalendarEvents } from './calendar-central-test-utils';
import { member1Email, member1ID } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const eventTitle = '팀 상태 종일 행사';
const eventDay = dayOfMonth(seoulMonthToday(), 10);
const dayBeforeEvent = dayOfMonth(seoulMonthToday(), 9);

let seededEventIDs: string[] = [];

test.beforeAll(async () => {
	seededEventIDs = await seedCalendarEvents([
		{
			title: eventTitle,
			startISO: seoulInstant(eventDay, '00:00'),
			endISO: seoulInstant(dayOfMonth(seoulMonthToday(), 11), '00:00'),
			isAllDay: true,
			participantIDs: [member1ID]
		}
	]);
});

test.afterAll(async () => {
	await cleanupCalendarEvents(seededEventIDs);
});

function dayCell(page: Page, date: string) {
	return page.getByTestId(`team-status-cell-${member1Email}-${date}`);
}

async function openDayDetail(page: Page, date: string): Promise<void> {
	await signInToAttendance(page);
	await page.getByTestId('team-status-table').waitFor({ state: 'visible', timeout: 20000 });
	await dayCell(page, date).waitFor({ state: 'visible', timeout: 20000 });
	await dayCell(page, date).click();
	await page.getByTestId('team-status-day-detail-dialog').waitFor({ state: 'visible', timeout: 20000 });
}

test('a whole-day event appears on the company day it covers', async ({ page }) => {
	await openDayDetail(page, eventDay);

	const detail = page.getByTestId('team-status-day-detail-dialog');
	await expect(detail.getByTestId('team-status-calendar-event')).toContainText(eventTitle);
});

test('a whole-day event does not appear one day early', async ({ page }) => {
	await openDayDetail(page, dayBeforeEvent);

	const detail = page.getByTestId('team-status-day-detail-dialog');
	await expect(detail.getByTestId('calendar-empty-state')).toBeVisible();
	await expect(detail.getByTestId('team-status-calendar-event')).toHaveCount(0);
});

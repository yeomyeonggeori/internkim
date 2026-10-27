import { expect, test } from '@playwright/test';
import {
	cleanupAttendanceEvents,
	dayOfMonth,
	openMonthlyAttendance,
	seedAttendanceEvents,
	seoulInstant,
	seoulMonthToday,
	signInToAttendance
} from './attendance-central-test-utils';
import { member1Email, member1ID } from '../support/central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR', viewport: { width: 390, height: 844 } });

const recordDay = dayOfMonth(seoulMonthToday(), 12);

let seededEventIDs: string[] = [];

test.beforeAll(async () => {
	seededEventIDs = await seedAttendanceEvents([
		{ memberID: member1ID, kind: 'clock_in', occurredAtISO: seoulInstant(recordDay, '09:00') },
		{ memberID: member1ID, kind: 'clock_out', occurredAtISO: seoulInstant(recordDay, '18:00') }
	]);
});

test.afterAll(async () => {
	await cleanupAttendanceEvents(seededEventIDs);
});

test('the remove confirmation covers the day detail it was opened from', async ({ page }) => {
	await signInToAttendance(page);
	await openMonthlyAttendance(page);
	await page.getByTestId(`team-status-cell-${member1Email}-${recordDay}`).click();
	const detail = page.getByTestId('team-status-day-detail-sheet');
	await detail.waitFor({ state: 'visible', timeout: 20000 });
	const editButton = detail.getByTestId('work-record-edit-button');
	await expect(async () => {
		if ((await editButton.getAttribute('data-state')) !== 'editing') await editButton.click();
		await expect(editButton).toHaveAttribute('data-state', 'editing', { timeout: 1000 });
	}).toPass({ timeout: 20000 });
	await detail.getByTestId('work-record-remove-start').click();
	await expect(page.getByTestId('attendance-record-remove-submit')).toBeVisible();

	const header = await detail.getByTestId('team-status-work-record-header').boundingBox();
	if (!header) throw new Error('The day detail header has no box to probe.');
	const topmostSlot = await page.evaluate(
		({ x, y }) => document.elementFromPoint(x, y)?.closest('[data-slot]')?.getAttribute('data-slot') ?? null,
		{ x: header.x + 4, y: header.y + 4 }
	);
	expect(topmostSlot).toBe('alert-dialog-overlay');
});

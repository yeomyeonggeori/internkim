import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import {
	attendanceRowsOf,
	cleanupAttendanceEvents,
	dayOfMonth,
	monthBefore,
	removeAttendanceOf,
	seedAttendanceEvents,
	seoulInstant,
	seoulMonthToday
} from './attendance-central-test-utils';
import { member1Email, member3Email, member3ID, member3Name } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const writtenDate = dayOfMonth(monthBefore(seoulMonthToday()), 15);
const movedTo = '10:00';
const writtenAt = '09:00';
const writtenReason = 'time_correction';

let seededEventIDs: string[] = [];

test.beforeAll(async () => {
	await removeAttendanceOf(member3ID);
	seededEventIDs = await seedAttendanceEvents([
		{
			memberID: member3ID,
			kind: 'clock_in',
			location: '사무실',
			occurredAtISO: seoulInstant(writtenDate, movedTo),
			originalOccurredAtISO: seoulInstant(writtenDate, writtenAt),
			editReason: writtenReason
		}
	]);
});

test.afterAll(async () => {
	await cleanupAttendanceEvents(seededEventIDs);
});

async function openHandWrittenRecords(page: Page, email: string): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/attendance', email);
	await page.getByTestId('attendance-sidebar-scroll').waitFor({ state: 'visible', timeout: 30000 });
	await page.getByTestId('hand-written-navigation').click();
	await page.getByTestId('hand-written-view').waitFor({ state: 'visible', timeout: 20000 });
}

function seededRow(page: Page) {
	return page.locator(`[data-testid="hand-written-row"][data-event-id="${seededEventIDs[0]}"]`);
}

test('an administrator sees what was written by hand and undoes it from the list', async ({
	page
}) => {
	await openHandWrittenRecords(page, member1Email);

	const row = seededRow(page);
	await expect(row).toBeVisible();
	await expect(row).toContainText(member3Name);
	await expect(row).toContainText('시간 정정');
	await expect(row.getByTestId('hand-written-now')).toContainText(new RegExp(`${writtenDate}\\s*${movedTo}`));
	await expect(row.getByTestId('hand-written-before')).toContainText(
		new RegExp(`${writtenDate}\\s*${writtenAt}`)
	);

	await row.getByTestId('hand-written-undo').click();
	await page.getByTestId('hand-written-undo-dialog').waitFor({ state: 'visible' });
	await page.getByTestId('hand-written-undo-confirm').click();

	await expect(row.getByTestId('hand-written-now')).toContainText(
		new RegExp(`${writtenDate}\\s*${writtenAt}`),
		{ timeout: 20000 }
	);
	await expect
		.poll(
			async () => {
				const [held] = await attendanceRowsOf(member3ID);
				return held ? new Date(held.occurred_at).getTime() : 0;
			},
			{ timeout: 20000 }
		)
		.toBe(new Date(seoulInstant(writtenDate, writtenAt)).getTime());
});

test('somebody who does not administer the company never sees the list', async ({ page }) => {
	await signInToTheCentralPlane(page, '/example-co/attendance', member3Email);
	await page.getByTestId('attendance-sidebar-scroll').waitFor({ state: 'visible', timeout: 30000 });

	await expect(page.getByTestId('hand-written-navigation')).toHaveCount(0);
	await expect(page.getByTestId('hand-written-view')).toHaveCount(0);
});

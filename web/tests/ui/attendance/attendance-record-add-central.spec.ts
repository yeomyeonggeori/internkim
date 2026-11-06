import { expect, test } from '@playwright/test';
import { dayOfMonth, openMonthlyAttendance, seoulMonthToday, signInToAttendance } from './attendance-central-test-utils';
import { member1Email } from '../support/central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR', viewport: { width: 390, height: 844 } });

const recordDay = dayOfMonth(seoulMonthToday(), 12);

test('the add form covers the day detail it was opened from', async ({ page }) => {
	await signInToAttendance(page);
	await openMonthlyAttendance(page);
	await page.getByTestId(`team-status-cell-${member1Email}-${recordDay}`).click();
	const detail = page.getByTestId('team-status-day-detail-sheet');
	await detail.waitFor({ state: 'visible', timeout: 20000 });
	const addButton = detail.getByTestId('work-record-add-button');
	const submit = page.getByTestId('attendance-record-add-submit');
	await expect(async () => {
		if (!(await submit.isVisible())) await addButton.click();
		await expect(submit).toBeVisible({ timeout: 1000 });
	}).toPass({ timeout: 20000 });

	const header = await detail.getByTestId('team-status-work-record-header').boundingBox();
	if (!header) throw new Error('The day detail header has no box to probe.');
	const topmostSlot = await page.evaluate(
		({ x, y }) => document.elementFromPoint(x, y)?.closest('[data-slot]')?.getAttribute('data-slot') ?? null,
		{ x: 4, y: header.y + 4 }
	);
	expect(topmostSlot).toBe('dialog-overlay');
});

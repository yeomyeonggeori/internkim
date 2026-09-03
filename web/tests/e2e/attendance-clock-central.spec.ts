import { expect, test, type Page } from '@playwright/test';
import {
	attendanceRowsOf,
	removeAttendanceOf,
	signInToAttendance
} from './attendance-central-test-utils';
import { member1ID } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const office = '사무실';
const home = '재택';

test.beforeAll(async () => {
	await removeAttendanceOf(member1ID);
});

test.afterAll(async () => {
	await removeAttendanceOf(member1ID);
});

function quickActions(page: Page) {
	return page.getByTestId('personal-tools-panel');
}

async function openCommandPalette(page: Page): Promise<void> {
	await page.keyboard.press('Slash');
	await page.getByRole('dialog').getByRole('textbox').waitFor({ state: 'visible' });
}

async function openClockMenu(page: Page): Promise<void> {
	await page.keyboard.press('Period');
	await page.locator('[data-app-rail-profile-menu]').waitFor({ state: 'visible' });
}

async function recordedClockKinds(): Promise<string[]> {
	return (await attendanceRowsOf(member1ID)).map((row) => row.kind);
}

async function recordedLocations(): Promise<(string | null)[]> {
	return (await attendanceRowsOf(member1ID)).map((row) => row.location);
}

test('the command palette clocks in at the location it names', async ({ page }) => {
	await signInToAttendance(page);
	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible();

	await openCommandPalette(page);
	await page.locator(`[data-value="clock-in-${home}"]`).click();

	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in']);
	await expect.poll(recordedLocations).toEqual([home]);
	await expect(quickActions(page).getByText(home)).toBeVisible({ timeout: 20000 });
	await expect(quickActions(page).getByRole('button', { name: '퇴근', exact: true })).toBeVisible();
});

test('the clock rail clocks out and the record keeps the pair', async ({ page }) => {
	await signInToAttendance(page);

	await openClockMenu(page);
	await page.getByRole('menuitem', { name: '퇴근', exact: true }).click();

	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in', 'clock_out']);
	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible({
		timeout: 20000
	});
});

test('the clock rail clocks in at the location the menu offers', async ({ page }) => {
	await signInToAttendance(page);

	await openClockMenu(page);
	await page.getByRole('menuitemradio', { name: office, exact: true }).click();

	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual([
		'clock_in',
		'clock_out',
		'clock_in'
	]);
	await expect.poll(recordedLocations).toEqual([home, null, office]);
	await expect(quickActions(page).getByRole('button', { name: '퇴근', exact: true })).toBeVisible({
		timeout: 20000
	});
});

test('the command palette clock-out shortcut records the clock-out', async ({ page }) => {
	await signInToAttendance(page);

	await openCommandPalette(page);
	await page.keyboard.press('0');

	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual([
		'clock_in',
		'clock_out',
		'clock_in',
		'clock_out'
	]);
	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible({
		timeout: 20000
	});
});

test('the command palette offers no clock-out while nobody is clocked in', async ({ page }) => {
	await signInToAttendance(page);

	await openCommandPalette(page);

	await expect(page.locator('[data-value="clock-out"]')).toHaveAttribute('data-disabled', 'true');
	await expect(page.locator(`[data-value="clock-in-${office}"]`)).toBeVisible();
});

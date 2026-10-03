import { expect, test, type Page } from '@playwright/test';
import {
	attendanceRowsOf,
	removeAttendanceOf,
	seedAttendanceEvents,
	seoulDateToday,
	seoulInstant,
	signInToAttendance
} from './attendance-central-test-utils';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { centralPlaneAdminClient, member1ID } from './central-test-utils';
import { moveAttendanceEarlier } from '../support/move-attendance-earlier';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const office = '사무실';
const home = '재택';
const minutesPastTakingBack = 2;

test.beforeAll(async () => {
	await removeAttendanceOf(member1ID);
});

test.afterAll(async () => {
	await removeAttendanceOf(member1ID);
});

function quickActions(page: Page) {
	return page.getByTestId('personal-tools-panel');
}

function trackToolInvokes(page: Page): { names: string[]; stop: () => void } {
	const names: string[] = [];
	const recordRequest = (request: { url(): string }) => {
		const match = request.url().match(/\/api\/v1\/tools\/([^/]+)\/invoke$/);
		if (match) names.push(match[1]);
	};
	page.on('request', recordRequest);
	return { names, stop: () => page.off('request', recordRequest) };
}

function paletteItem(page: Page, value: string) {
	return page.locator(`[role="option"][data-value="${value}"]`);
}

async function openCommandPalette(page: Page, offersClockOut: boolean): Promise<void> {
	await page.keyboard.press('Slash');
	await page.locator('[data-slot="command-input"]').waitFor({ state: 'visible' });
	const clockOut = paletteItem(page, 'clock-out');
	if (offersClockOut) {
		await expect(clockOut).not.toHaveAttribute('aria-disabled', 'true', { timeout: 20000 });
		return;
	}
	await expect(clockOut).toHaveAttribute('aria-disabled', 'true', { timeout: 20000 });
}

async function openClockMenu(page: Page, offersClockOut = true): Promise<void> {
	await page.keyboard.press('Period');
	const menu = page.locator('[data-app-rail-profile-menu]');
	await menu.waitFor({ state: 'visible' });
	const clockOut = menu.getByRole('menuitem', { name: '퇴근', exact: true });
	if (offersClockOut) {
		await expect(clockOut).not.toHaveAttribute('aria-disabled', 'true', { timeout: 20000 });
		return;
	}
	await expect(clockOut).toHaveAttribute('aria-disabled', 'true', { timeout: 20000 });
}

async function recordedClockKinds(): Promise<string[]> {
	return (await attendanceRowsOf(member1ID)).map((row) => row.kind);
}

async function recordedLocations(): Promise<(string | null)[]> {
	return (await attendanceRowsOf(member1ID)).map((row) => row.location);
}

test('the first clock menu opens an existing clock-in without an eager task read', async ({ page }) => {
	await seedAttendanceEvents([
		{ memberID: member1ID, kind: 'clock_in', occurredAtISO: seoulInstant(seoulDateToday(), '09:00'), location: home }
	]);
	try {
		const invokes = trackToolInvokes(page);
		await signInToTheCentralPlane(page, '/example-co/task');
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await page.waitForLoadState('networkidle');
		expect(invokes.names).not.toContain('attendance_list');
		expect(invokes.names).not.toContain('attendance_current_get');
		const attendanceLoad = page.waitForResponse(
			(response) => response.url().includes('/api/v1/tools/attendance_current_get/invoke') && response.ok(),
			{ timeout: 30000 }
		);
		await page.keyboard.press('Period');
		await attendanceLoad;
		await expect(page.locator('[data-app-rail-profile-menu]').getByRole('menuitem', { name: '퇴근', exact: true })).not.toHaveAttribute('aria-disabled', 'true', { timeout: 20000 });
		invokes.stop();
	} finally {
		await removeAttendanceOf(member1ID);
	}
});

test('the command palette clocks in at the location it names', async ({ page }) => {
	await signInToAttendance(page);
	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible();

	await openCommandPalette(page, false);
	await page.waitForLoadState('networkidle');
	const invokes = trackToolInvokes(page);
	await paletteItem(page, `clock-in-${home}`).click();

	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in']);
	await expect.poll(recordedLocations).toEqual([home]);
	await expect(quickActions(page).getByText(home)).toBeVisible({ timeout: 20000 });
	await expect(quickActions(page).getByRole('button', { name: '퇴근', exact: true })).toBeVisible();
	invokes.stop();
	expect(invokes.names).toEqual(['attendance_add']);
});

test('the clock rail clocks out and the record keeps the pair', async ({ page }) => {
	await moveAttendanceEarlier(centralPlaneAdminClient(), member1ID, minutesPastTakingBack);
	await signInToAttendance(page);

	await openClockMenu(page);
	await page.waitForLoadState('networkidle');
	const invokes = trackToolInvokes(page);
	await page.getByRole('menuitem', { name: '퇴근', exact: true }).click();

	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in', 'clock_out']);
	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible({
		timeout: 20000
	});
	invokes.stop();
	expect(invokes.names).toEqual(['attendance_add']);
});

test('the clock rail clocks in at the location the menu offers', async ({ page }) => {
	await signInToAttendance(page);

	await openClockMenu(page, false);
	await page.waitForLoadState('networkidle');
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
	await moveAttendanceEarlier(centralPlaneAdminClient(), member1ID, minutesPastTakingBack);
	await signInToAttendance(page);

	await openCommandPalette(page, true);
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

	await openCommandPalette(page, false);
	await page.waitForLoadState('networkidle');

	await expect(paletteItem(page, 'clock-out')).toHaveAttribute('aria-disabled', 'true');
	await expect(paletteItem(page, `clock-in-${office}`)).toBeVisible();
});

test('a failed clock-in leaves the original action available', async ({ page }) => {
	await removeAttendanceOf(member1ID);
	await signInToAttendance(page);
	await page.route('**/api/v1/tools/attendance_add/invoke', async (route) =>
		route.fulfill({ status: 422, json: { error: 'clock unavailable' } })
	);

	await openCommandPalette(page, false);
	await paletteItem(page, `clock-in-${home}`).click();

	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible();
	await expect(page.getByText('기록하지 못했습니다').last()).toBeVisible();
	await expect.poll(recordedClockKinds).toEqual([]);
	await page.unroute('**/api/v1/tools/attendance_add/invoke');
});

test('a clock-out within a minute of the clock-in says the clock-in was taken back', async ({ page }) => {
	await removeAttendanceOf(member1ID);
	await signInToAttendance(page);
	await openCommandPalette(page, false);
	await paletteItem(page, `clock-in-${home}`).click();
	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in']);

	await openClockMenu(page);
	await page.getByRole('menuitem', { name: '퇴근', exact: true }).click();

	await expect(page.getByText('방금 누른 출근을 취소했습니다')).toBeVisible({ timeout: 20000 });
	await expect(quickActions(page).getByRole('button', { name: '출근', exact: true })).toBeVisible();
});

test('a clock-in within a minute of the clock-out says the work goes on', async ({ page }) => {
	await removeAttendanceOf(member1ID);
	await signInToAttendance(page);
	await openCommandPalette(page, false);
	await paletteItem(page, `clock-in-${home}`).click();
	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in']);
	await moveAttendanceEarlier(centralPlaneAdminClient(), member1ID, minutesPastTakingBack);
	await signInToAttendance(page);

	await openClockMenu(page);
	await page.waitForLoadState('networkidle');
	await page.getByRole('menuitem', { name: '퇴근', exact: true }).click();
	await expect.poll(recordedClockKinds, { timeout: 20000 }).toEqual(['clock_in', 'clock_out']);
	await openClockMenu(page, false);
	await page.getByRole('menuitemradio', { name: home, exact: true }).click();

	await expect(page.getByText('방금 누른 퇴근을 취소하고 근무를 이어갑니다')).toBeVisible({ timeout: 20000 });
	await expect(quickActions(page).getByRole('button', { name: '퇴근', exact: true })).toBeVisible();
});

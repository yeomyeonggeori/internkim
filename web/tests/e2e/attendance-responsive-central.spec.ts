import { expect, test, type Page } from '@playwright/test';
import {
	cleanupAttendanceEvents,
	renameMember,
	seedAttendanceEvents,
	seoulDateToday,
	seoulInstant,
	signInToAttendance
} from './attendance-central-test-utils';
import {
	expectReadableMobileTeamStatusTable,
	measureMobileTeamStatusTable,
	mobileMonthPickerLayout,
	mobileTabListLayout,
	mobileTabStyle
} from './attendance-responsive-helpers';
import { member1Email, member1ID, member1Name } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

const mobileViewport = { width: 390, height: 844 };
const desktopViewport = { width: 1280, height: 900 };
const longDisplayName = '아주긴이름테스트사용자';
const office = '사무실';
const today = seoulDateToday();

let seededEventIDs: string[] = [];

test.beforeAll(async () => {
	await renameMember(member1ID, longDisplayName);
	seededEventIDs = await seedAttendanceEvents([
		{ memberID: member1ID, kind: 'clock_in', location: office, occurredAtISO: seoulInstant(today, '00:30') }
	]);
});

test.afterAll(async () => {
	await cleanupAttendanceEvents(seededEventIDs);
	await renameMember(member1ID, member1Name);
});

function statusTable(page: Page) {
	return page.getByTestId('team-status-table');
}

function tabList(page: Page) {
	return page.getByRole('tablist');
}

async function openMobileTeamStatus(page: Page): Promise<void> {
	await page.setViewportSize(mobileViewport);
	await signInToAttendance(page);
	await tabList(page).getByRole('tab', { name: '팀 현황' }).click();
	await statusTable(page).waitFor({ state: 'visible', timeout: 30000 });
	await page
		.getByTestId(`team-status-cell-${member1Email}-${today}`)
		.waitFor({ state: 'visible', timeout: 30000 });
}

test('the mobile layout swaps between the personal tools and the team status', async ({ page }) => {
	await page.setViewportSize(mobileViewport);
	await signInToAttendance(page);

	const recordsTab = tabList(page).getByRole('tab', { name: '내 기록' });
	const statusTab = tabList(page).getByRole('tab', { name: '팀 현황' });
	await expect(recordsTab).toHaveAttribute('data-state', 'active');
	await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();
	await expect(page.getByTestId('team-status-grid')).toBeHidden();

	const tabColors = await mobileTabStyle(recordsTab, statusTab);
	expect(tabColors.recordsBackground).not.toBe(tabColors.statusBackground);

	const layout = await mobileTabListLayout(tabList(page));
	expect(layout.left).toBeGreaterThanOrEqual(0);
	expect(layout.right).toBeLessThanOrEqual(layout.viewportWidth);

	await statusTab.click();
	await expect(page.getByTestId('team-status-grid')).toBeVisible({ timeout: 20000 });
	await expect(page.getByTestId('mobile-attendance-tools-view')).toBeHidden();
});

test('the mobile tabs reach the leave surfaces an administrator has', async ({ page }) => {
	await page.setViewportSize(mobileViewport);
	await signInToAttendance(page);

	await tabList(page).getByRole('tab', { name: '휴가 내역' }).click();
	await expect(page.getByTestId('leave-history-view')).toBeVisible({ timeout: 20000 });

	await tabList(page).getByRole('tab', { name: '휴가 승인' }).click();
	await expect(page.getByTestId('leave-approval-view')).toBeVisible({ timeout: 20000 });

	await tabList(page).getByRole('tab', { name: '직원별 휴가' }).click();
	await expect(page.getByTestId('leave-management-view')).toBeVisible({ timeout: 20000 });
});

test('the mobile month picker stays inside the employee filter', async ({ page }) => {
	await openMobileTeamStatus(page);

	const layout = await mobileMonthPickerLayout(page);

	expect(layout.previousGap).toBeGreaterThanOrEqual(0);
	expect(layout.nextGap).toBeGreaterThanOrEqual(0);
	expect(layout.nextButtonRight).toBeLessThanOrEqual(layout.filterRight + 1);
});

test('the mobile status table stays readable while it scrolls', async ({ page }) => {
	await openMobileTeamStatus(page);

	const measured = await measureMobileTeamStatusTable(statusTable(page), {
		email: member1Email,
		date: today
	});

	expectReadableMobileTeamStatusTable(measured, office);
});

test('the desktop layout keeps the personal tools in the scrolling sidebar', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 500 });
	await signInToAttendance(page);

	await expect(tabList(page)).toHaveCount(0);
	await expect(page.getByTestId('team-status-grid')).toBeVisible({ timeout: 20000 });

	const sidebarScroll = page.getByTestId('attendance-sidebar-scroll');
	const scrolling = await sidebarScroll.evaluate((element) => ({
		overflowY: getComputedStyle(element).overflowY,
		canScroll: element.scrollHeight > element.clientHeight
	}));
	expect(scrolling.overflowY).toBe('auto');
	expect(scrolling.canScroll).toBe(true);
	await expect(sidebarScroll.getByTestId('personal-tools-panel')).toBeVisible();
});

test('widening the window returns the team status table', async ({ page }) => {
	await page.setViewportSize(mobileViewport);
	await signInToAttendance(page);
	await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();

	await page.setViewportSize(desktopViewport);

	await expect(tabList(page)).toHaveCount(0);
	await expect(page.getByTestId('team-status-grid')).toBeVisible({ timeout: 20000 });
});

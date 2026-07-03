import { expect, type Locator, type Page } from '@playwright/test';
import type { AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';

type MobileTeamStatusTableLayout = {
	canScrollDates: boolean;
	initialEmployeeLeft: number;
	initialEmployeeRowLeft: number;
	initialTargetLeft: number;
	scrolledEmployeeLeft: number;
	scrolledEmployeeRowLeft: number;
	scrolledTargetLeft: number;
	employeeColumnWidth: number;
	employeeRowHeight: number;
	tableLeft: number;
	overflowingLabels: string[];
	overflowingEmployeeDetails: string[];
	visibleLocationLabels: string[];
	pageOverflows: boolean;
};

type MobileMonthPickerLayoutOptions = {
	searchPlaceholder: string;
	previousMonthLabel: string;
	monthTriggerName: RegExp;
	nextMonthLabel: string;
};

const koreanMonthPickerLayoutOptions: MobileMonthPickerLayoutOptions = {
	searchPlaceholder: '직원 검색',
	previousMonthLabel: '이전 달',
	monthTriggerName: /\d{4}년 \d+월/,
	nextMonthLabel: '다음 달'
};

export async function measureMobileTeamStatusTable(
	statusTable: Locator,
	targetDate: string
): Promise<MobileTeamStatusTableLayout> {
	return statusTable.evaluate((tableElement, date) => {
		const table = tableElement as HTMLElement;
		const employeeHeader = table.querySelector<HTMLElement>('[role="columnheader"]');
		const employeeRowHeader = table.querySelector<HTMLElement>('[role="rowheader"]');
		const targetCell = table.querySelector<HTMLElement>(`[data-testid="team-status-cell-kim@example.com-${date}"]`);
		if (!employeeHeader || !employeeRowHeader || !targetCell) {
			throw new Error('Missing monthly attendance table cells');
		}

		table.scrollLeft = 0;
		const initialEmployeeLeft = Math.round(employeeHeader.getBoundingClientRect().left);
		const initialEmployeeRowLeft = Math.round(employeeRowHeader.getBoundingClientRect().left);
		const initialTargetLeft = Math.round(targetCell.getBoundingClientRect().left);
		const employeeColumnWidth = Math.round(employeeHeader.getBoundingClientRect().width);
		const employeeRowHeight = Math.round(employeeRowHeader.getBoundingClientRect().height);
		const tableLeft = Math.round(table.getBoundingClientRect().left);
		table.scrollLeft = 480;
		const scrolledEmployeeLeft = Math.round(employeeHeader.getBoundingClientRect().left);
		const scrolledEmployeeRowLeft = Math.round(employeeRowHeader.getBoundingClientRect().left);
		const scrolledTargetLeft = Math.round(targetCell.getBoundingClientRect().left);
		const overflowingLabels = Array.from(
			table.querySelectorAll<HTMLButtonElement>('button[data-testid^="team-status-cell-"]')
		)
			.map((button) => button.firstElementChild)
			.filter((label): label is HTMLElement => label instanceof HTMLElement)
			.filter((label) => label.scrollWidth > label.clientWidth + 1)
			.map((label) => label.textContent?.trim() ?? '');
		const overflowingEmployeeDetails = Array.from(table.querySelectorAll<HTMLElement>('[role="rowheader"] span'))
			.filter((label) => label.scrollWidth > label.clientWidth + 1)
			.map((label) => label.textContent?.trim() ?? '');
		const visibleLocationLabels = Array.from(table.querySelectorAll<HTMLElement>('[data-testid="team-status-current-location"]'))
			.filter((label) => label.offsetParent !== null)
			.map((label) => label.textContent?.trim() ?? '');

		return {
			canScrollDates: table.scrollWidth > table.clientWidth,
			initialEmployeeLeft,
			initialEmployeeRowLeft,
			initialTargetLeft,
			scrolledEmployeeLeft,
			scrolledEmployeeRowLeft,
			scrolledTargetLeft,
			employeeColumnWidth,
			employeeRowHeight,
			tableLeft,
			overflowingLabels,
			overflowingEmployeeDetails,
			visibleLocationLabels,
			pageOverflows: document.documentElement.scrollWidth > window.innerWidth
		};
	}, targetDate);
}

export function expectReadableMobileTeamStatusTable(layout: MobileTeamStatusTableLayout): void {
	expect(layout.canScrollDates).toBe(true);
	expect(Math.abs(layout.scrolledEmployeeLeft - layout.tableLeft)).toBeLessThanOrEqual(1);
	expect(Math.abs(layout.scrolledEmployeeRowLeft - layout.tableLeft)).toBeLessThanOrEqual(1);
	expect(Math.abs(layout.scrolledEmployeeRowLeft - layout.initialEmployeeRowLeft)).toBeLessThanOrEqual(1);
	expect(layout.employeeColumnWidth).toBeLessThanOrEqual(224);
	expect(layout.employeeRowHeight).toBeLessThanOrEqual(72);
	expect(layout.scrolledTargetLeft).toBeLessThan(layout.initialTargetLeft);
	expect(layout.overflowingLabels).toEqual([]);
	expect(layout.overflowingEmployeeDetails).toEqual([]);
	expect(layout.visibleLocationLabels).toContain('사무실본관회의실A');
	expect(layout.pageOverflows).toBe(false);
}

export function withLongMobileDisplayName(summary: AttendanceSummary): AttendanceSummary {
	return {
		...summary,
		events: summary.events.map((event) =>
			event.email === 'kim@example.com' ? { ...event, displayName: '아주긴이름테스트사용자' } : event
		)
	};
}

export async function mobileTabStyle(
	recordsTab: Locator,
	statusTab: Locator
): Promise<{ recordsBackground: string; statusBackground: string }> {
	const [recordsBackground, statusBackground] = await Promise.all([
		recordsTab.evaluate((element) => getComputedStyle(element).backgroundColor),
		statusTab.evaluate((element) => getComputedStyle(element).backgroundColor)
	]);
	return { recordsBackground, statusBackground };
}

export async function mobileTabListLayout(tabList: Locator): Promise<{ left: number; width: number }> {
	return tabList.evaluate((element) => {
		const rect = element.getBoundingClientRect();
		return {
			left: Math.round(rect.left),
			width: Math.round(rect.width)
		};
	});
}

export async function mobileMonthPickerLayout(
	page: Page,
	options = koreanMonthPickerLayoutOptions
): Promise<{
	searchRight: number;
	previousGap: number;
	nextGap: number;
	nextButtonRight: number;
}> {
	const searchInput = page.getByPlaceholder(options.searchPlaceholder);
	const previousMonthButton = page.getByRole('button', { name: options.previousMonthLabel });
	const monthTrigger = page.getByRole('button', { name: options.monthTriggerName });
	const nextMonthButton = page.getByRole('button', { name: options.nextMonthLabel });
	const [searchBox, previousBox, triggerBox, nextBox] = await Promise.all([
		searchInput.boundingBox(),
		previousMonthButton.boundingBox(),
		monthTrigger.boundingBox(),
		nextMonthButton.boundingBox()
	]);
	if (!searchBox || !previousBox || !triggerBox || !nextBox) {
		throw new Error('Mobile month picker layout could not be measured');
	}
	return {
		searchRight: Math.round(searchBox.x + searchBox.width),
		previousGap: Math.round(triggerBox.x - (previousBox.x + previousBox.width)),
		nextGap: Math.round(nextBox.x - (triggerBox.x + triggerBox.width)),
		nextButtonRight: Math.round(nextBox.x + nextBox.width)
	};
}

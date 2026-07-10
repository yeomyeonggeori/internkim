import { expect, type Locator, type Page } from '@playwright/test';
import type { AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';

type MobileTeamStatusTableLayout = {
	canScrollDates: boolean;
	canScrollEmployees: boolean;
	initialEmployeeHeaderTop: number;
	scrolledEmployeeHeaderTop: number;
	initialDateRowHeaderLeft: number;
	scrolledDateRowHeaderLeft: number;
	initialTargetTop: number;
	scrolledTargetTop: number;
	employeeColumnWidth: number;
	dateRowHeight: number;
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
		const employeeHeader = table.querySelector<HTMLElement>('[data-testid^="team-status-person-header-"]');
		const dateRowHeader = table.querySelector<HTMLElement>(`[data-testid="team-status-day-${date}"]`);
		const targetCell = table.querySelector<HTMLElement>(`[data-testid="team-status-cell-kim@example.com-${date}"]`);
		if (!employeeHeader || !dateRowHeader || !targetCell) {
			throw new Error(`Missing monthly attendance table cells: employeeHeader=${!!employeeHeader}, dateRowHeader=${!!dateRowHeader}, targetCell=${!!targetCell}`);
		}

		table.scrollTop = 0;
		table.scrollLeft = 0;
		const initialEmployeeHeaderTop = Math.round(employeeHeader.getBoundingClientRect().top);
		const initialDateRowHeaderLeft = Math.round(dateRowHeader.getBoundingClientRect().left);
		const initialTargetTop = Math.round(targetCell.getBoundingClientRect().top);
		const employeeColumnWidth = Math.round(employeeHeader.getBoundingClientRect().width);
		const dateRowHeight = Math.round(dateRowHeader.getBoundingClientRect().height);
		const canScrollDates = table.scrollHeight > table.clientHeight;
		const canScrollEmployees = table.scrollWidth > table.clientWidth;

		table.scrollTop = 480;
		const scrolledEmployeeHeaderTop = Math.round(employeeHeader.getBoundingClientRect().top);
		const scrolledTargetTop = Math.round(targetCell.getBoundingClientRect().top);

		table.scrollLeft = canScrollEmployees ? 80 : 0;
		const scrolledDateRowHeaderLeft = Math.round(dateRowHeader.getBoundingClientRect().left);

		const overflowingLabels = Array.from(
			table.querySelectorAll<HTMLButtonElement>('button[data-testid^="team-status-cell-"]')
		)
			.map((button) => button.firstElementChild)
			.filter((label): label is HTMLElement => label instanceof HTMLElement)
			.filter((label) => label.scrollWidth > label.clientWidth + 1)
			.map((label) => label.textContent?.trim() ?? '');
		const overflowingEmployeeDetails = Array.from(
			table.querySelectorAll<HTMLElement>('[role="columnheader"] div, [role="columnheader"] span')
		)
			.filter((label) => label.scrollWidth > label.clientWidth + 1)
			.map((label) => label.textContent?.trim() ?? '');
		const visibleLocationLabels = Array.from(table.querySelectorAll<HTMLElement>('[data-testid="team-status-current-location"]'))
			.filter((label) => label.offsetParent !== null)
			.map((label) => label.textContent?.trim() ?? '');

		return {
			canScrollDates,
			canScrollEmployees,
			initialEmployeeHeaderTop,
			scrolledEmployeeHeaderTop,
			initialDateRowHeaderLeft,
			scrolledDateRowHeaderLeft,
			initialTargetTop,
			scrolledTargetTop,
			employeeColumnWidth,
			dateRowHeight,
			overflowingLabels,
			overflowingEmployeeDetails,
			visibleLocationLabels,
			pageOverflows: document.documentElement.scrollWidth > window.innerWidth
		};
	}, targetDate);
}

export function expectReadableMobileTeamStatusTable(layout: MobileTeamStatusTableLayout): void {
	expect(layout.canScrollDates).toBe(true);
	expect(Math.abs(layout.scrolledEmployeeHeaderTop - layout.initialEmployeeHeaderTop)).toBeLessThanOrEqual(1);
	expect(Math.abs(layout.scrolledDateRowHeaderLeft - layout.initialDateRowHeaderLeft)).toBeLessThanOrEqual(1);
	expect(layout.scrolledTargetTop).toBeLessThan(layout.initialTargetTop);
	expect(layout.employeeColumnWidth).toBeLessThanOrEqual(224);
	expect(layout.dateRowHeight).toBeLessThanOrEqual(64);
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

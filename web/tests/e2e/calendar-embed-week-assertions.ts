import { expect, type Page } from '@playwright/test';

export async function expectWeekendGridStyle(page: Page): Promise<void> {
	const styles = await page.evaluate(() => {
		const header = document.querySelector('.df-week-header > .df-week-day-cell:first-child .df-date-number');
		const allDayCell = document.querySelector('.df-week-all-day-row-content > .df-week-all-day-cell:first-child, .df-all-day-row > .df-all-day-cell:first-child');
		const timeCell = document.querySelector('.df-time-grid-row > .df-week-time-grid-cell:first-child');
		if (!(header instanceof HTMLElement) || !(allDayCell instanceof HTMLElement) || !(timeCell instanceof HTMLElement)) return null;
		return {
			headerColor: window.getComputedStyle(header).color,
			allDayBackground: window.getComputedStyle(allDayCell).backgroundColor,
			timeCellBackground: window.getComputedStyle(timeCell).backgroundColor
		};
	});
	expect(styles).toEqual({
		headerColor: 'rgb(248, 113, 113)',
		allDayBackground: 'rgb(250, 250, 250)',
		timeCellBackground: 'rgb(250, 250, 250)'
	});
}

export async function expectWeekAllDayReferenceGrid(page: Page): Promise<void> {
	const measurements = await page.evaluate(() => {
		const shell = document.querySelector('.df-week-all-day-shell');
		const label = document.querySelector('.df-week-all-day-label, .df-all-day-label');
		const contentWrap = document.querySelector('.df-week-all-day-content-wrap');
		const firstCell = document.querySelector('.df-week-all-day-row-content > .df-week-all-day-cell:first-child, .df-all-day-row > .df-all-day-cell:first-child');
		const secondCell = document.querySelector('.df-week-all-day-row-content > .df-week-all-day-cell:nth-child(2), .df-all-day-row > .df-all-day-cell:nth-child(2)');
		const firstTimeCell = document.querySelector('.df-time-grid-row > .df-week-time-grid-cell:first-child');
		if (
			!(shell instanceof HTMLElement) ||
			!(label instanceof HTMLElement) ||
			!(contentWrap instanceof HTMLElement) ||
			!(firstCell instanceof HTMLElement) ||
			!(secondCell instanceof HTMLElement) ||
			!(firstTimeCell instanceof HTMLElement)
		) {
			return null;
		}
		const shellRectangle = shell.getBoundingClientRect();
		const labelRectangle = label.getBoundingClientRect();
		const contentRectangle = contentWrap.getBoundingClientRect();
		const cellRectangle = firstCell.getBoundingClientRect();
		const secondCellRectangle = secondCell.getBoundingClientRect();
		const firstTimeCellRectangle = firstTimeCell.getBoundingClientRect();
		const firstCellStyle = window.getComputedStyle(firstCell);
		return {
			shellHeight: Math.round(shellRectangle.height),
			labelHeight: Math.round(labelRectangle.height),
			contentHeight: Math.round(contentRectangle.height),
			cellHeight: Math.round(cellRectangle.height),
			cellTopGap: Math.round(cellRectangle.top - shellRectangle.top),
			firstDividerRight: Math.round(cellRectangle.right),
			secondCellLeft: Math.round(secondCellRectangle.left),
			firstTimeCellRight: Math.round(firstTimeCellRectangle.right),
			firstCellBorderRightColor: firstCellStyle.borderRightColor,
			firstCellBorderRightWidth: firstCellStyle.borderRightWidth
		};
	});
	expect(measurements).not.toBeNull();
	expect(measurements?.shellHeight).toBe(72);
	expect(measurements?.labelHeight).toBe(36);
	expect(measurements?.contentHeight).toBe(72);
	expect(measurements?.cellHeight).toBe(36);
	expect(Math.abs((measurements?.cellTopGap ?? 0) - 36)).toBeLessThanOrEqual(1);
	expect(Math.abs((measurements?.firstDividerRight ?? 0) - (measurements?.secondCellLeft ?? 0))).toBeLessThanOrEqual(1);
	expect(Math.abs((measurements?.firstDividerRight ?? 0) - (measurements?.firstTimeCellRight ?? 0))).toBeLessThanOrEqual(1);
	expect(measurements?.firstCellBorderRightColor).toBe('rgb(225, 229, 235)');
	expect(measurements?.firstCellBorderRightWidth).toBe('1px');
}

export async function expectWeekAllDayEventsCompactAndLabelCentered(page: Page): Promise<void> {
	await expect(page.locator('.df-week-all-day-event-layer .df-event:not(.calendar-multi-day-all-day-proxy)')).toHaveCount(5);
	const measurements = await page.evaluate(() => {
		const row = document.querySelector('.df-week-all-day-row-content, .df-all-day-row');
		const label = document.querySelector('.df-week-all-day-label, .df-all-day-label');
		if (!(row instanceof HTMLElement) || !(label instanceof HTMLElement)) return null;
		const rowRectangle = row.getBoundingClientRect();
		const labelRectangle = label.getBoundingClientRect();
		const eventRectangles = Array.from(
			document.querySelectorAll<HTMLElement>('.df-week-all-day-event-layer .df-event:not(.calendar-multi-day-all-day-proxy)')
		)
			.filter((element) => {
				const rectangle = element.getBoundingClientRect();
				const style = window.getComputedStyle(element);
				return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			})
			.map((element) => {
				const rectangle = element.getBoundingClientRect();
				return {
					height: Math.round(rectangle.height),
					top: Math.round(rectangle.top - rowRectangle.top)
				};
			})
			.sort((leftEvent, rightEvent) => leftEvent.top - rightEvent.top);
		const rowGaps = eventRectangles.slice(1).map((eventRectangle, index) => eventRectangle.top - eventRectangles[index].top);
		const labelCenter = labelRectangle.top + labelRectangle.height / 2;
		const rowCenter = rowRectangle.top + rowRectangle.height / 2;
		return {
			eventHeights: eventRectangles.map((eventRectangle) => eventRectangle.height),
			labelCenterGap: Math.round(Math.abs(labelCenter - rowCenter)),
			labelHeight: Math.round(labelRectangle.height),
			rowGaps,
			rowHeight: Math.round(rowRectangle.height),
			rowTops: eventRectangles.map((eventRectangle) => eventRectangle.top)
		};
	});
	expect(measurements).not.toBeNull();
	expect(measurements?.rowHeight).toBeGreaterThanOrEqual(106);
	expect(measurements?.labelHeight).toBe(measurements?.rowHeight);
	expect(measurements?.labelCenterGap).toBeLessThanOrEqual(1);
	expect(measurements?.eventHeights).toEqual([16, 16, 16, 16, 16]);
	expect(measurements?.rowGaps).toEqual([20, 20, 20, 20]);
}

export async function expectWeekAllDayExtendedDivider(page: Page, isVisible: boolean): Promise<void> {
	const hasExtendedDivider = await page.evaluate(() => {
		const cell = document.querySelector('.df-week-all-day-row-content > .df-week-all-day-cell:first-child, .df-all-day-row > .df-all-day-cell:first-child');
		if (!(cell instanceof HTMLElement)) return false;
		const style = window.getComputedStyle(cell, '::after');
		return style.content !== 'none' && style.content !== '' && Number.parseFloat(style.height) > 40;
	});
	expect(hasExtendedDivider).toBe(isVisible);
}

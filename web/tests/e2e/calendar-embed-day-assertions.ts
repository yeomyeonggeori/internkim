import { expect, type Page } from '@playwright/test';

export async function expectDayAllDayUsesContinuousTimelineBoundary(page: Page, shouldShowTick = true): Promise<void> {
	if (!shouldShowTick) {
		await expect
			.poll(async () =>
				page.evaluate(() => {
					const row = document.querySelector('.df-day-content-all-day-row');
					if (!(row instanceof HTMLElement)) return '';
					return window.getComputedStyle(row, '::after').content;
				})
			)
			.toBe('""');
	}

	const measurements = await page.evaluate(() => {
		const row = document.querySelector('.df-day-content-all-day-row');
		const label = document.querySelector('.df-day-content-all-day-label');
		const grid = document.querySelector('.df-day-content-grid');
		const timeAxis = document.querySelector('.df-time-column');
		const firstVisibleTimeSlot = document.querySelector('.df-time-column > .df-time-slot:nth-child(3)');
		const firstTimeGridRow = document.querySelector('.df-time-grid-row:first-child');
		if (!(row instanceof HTMLElement) || !(label instanceof HTMLElement) || !(grid instanceof HTMLElement) || !(timeAxis instanceof HTMLElement)) return null;
		const rowRectangle = row.getBoundingClientRect();
		const gridRectangle = grid.getBoundingClientRect();
		const rowStyle = window.getComputedStyle(row);
		const timeAxisStyle = window.getComputedStyle(timeAxis);
		const firstTimeGridRowStyle = firstTimeGridRow instanceof HTMLElement ? window.getComputedStyle(firstTimeGridRow) : null;
		const beforeStyle = window.getComputedStyle(row, '::before');
		const afterStyle = window.getComputedStyle(row, '::after');
		const labelAfterStyle = window.getComputedStyle(label, '::after');
		const firstVisibleTimeSlotBeforeStyle =
			firstVisibleTimeSlot instanceof HTMLElement ? window.getComputedStyle(firstVisibleTimeSlot, '::before') : null;
		return {
			afterContent: afterStyle.content,
			afterHeight: afterStyle.height,
			afterLeft: Math.round(Number.parseFloat(afterStyle.left) || 0),
			beforeContent: beforeStyle.content,
			beforeWidth: Math.round(Number.parseFloat(beforeStyle.width) || 0),
			boundaryGap: Math.round(gridRectangle.top - rowRectangle.bottom),
			firstTimeGridRowBorderTopColor: firstTimeGridRowStyle?.borderTopColor ?? '',
			firstTimeGridRowBorderTopWidth: firstTimeGridRowStyle?.borderTopWidth ?? '',
			labelBottom: Math.round(label.getBoundingClientRect().bottom),
			rowBottom: Math.round(rowRectangle.bottom),
			rowBorderBottomColor: rowStyle.borderBottomColor,
			rowBorderBottomWidth: rowStyle.borderBottomWidth,
			rowBorderTopWidth: rowStyle.borderTopWidth,
			timeAxisBorderRightColor: timeAxisStyle.borderRightColor,
			timeAxisBorderRightWidth: timeAxisStyle.borderRightWidth,
			tickContent: labelAfterStyle.content,
			timeSlotTickBorderColor: firstVisibleTimeSlotBeforeStyle?.borderTopColor ?? '',
			timeSlotTickBorderWidth: firstVisibleTimeSlotBeforeStyle?.borderTopWidth ?? ''
		};
	});
	expect(measurements).not.toBeNull();
	expect(['none', '']).toContain(measurements?.beforeContent);
	expect(measurements?.beforeWidth).toBeLessThanOrEqual(1);
	expect(Math.abs(measurements?.boundaryGap ?? Number.POSITIVE_INFINITY)).toBeLessThanOrEqual(1);
	expect(measurements?.firstTimeGridRowBorderTopWidth).toBe('1px');
	expect(measurements?.rowBorderTopWidth).toBe('0px');
	expect(measurements?.rowBorderBottomWidth).toBe('0px');
	expect(measurements?.timeAxisBorderRightWidth).toBe('1px');
	expect(measurements?.timeSlotTickBorderWidth).toBe('1px');
	expect(measurements?.timeAxisBorderRightColor).toBe(measurements?.firstTimeGridRowBorderTopColor);
	expect(measurements?.timeSlotTickBorderColor).toBe(measurements?.firstTimeGridRowBorderTopColor);
	expect(['none', '']).toContain(measurements?.tickContent);
	if (!shouldShowTick) {
		expect(measurements?.afterContent).toBe('""');
		expect(measurements?.afterHeight).toBe('1px');
		expect(measurements?.afterLeft).toBeLessThanOrEqual(1);
		return;
	}
	expect(['none', '']).toContain(measurements?.afterContent);
	expect(Math.abs((measurements?.labelBottom ?? 0) - (measurements?.rowBottom ?? 0))).toBeLessThanOrEqual(1);
}

export async function expectDayToolbarDividerUsesSingleBorder(page: Page): Promise<void> {
	const measurements = await page.evaluate(() => {
		const toolbar = document.querySelector('.calendar-toolbar');
		const rowElement = document.querySelector('.df-day-content-all-day-row');
		if (!(toolbar instanceof HTMLElement) || !(rowElement instanceof HTMLElement)) return null;
		const toolbarStyle = window.getComputedStyle(toolbar);
		const rowStyle = window.getComputedStyle(rowElement);
		return {
			rowBorderTopWidth: rowStyle.borderTopWidth,
			toolbarBorderBottomColor: toolbarStyle.borderBottomColor,
			toolbarBorderBottomWidth: toolbarStyle.borderBottomWidth
		};
	});
	expect(measurements).not.toBeNull();
	expect(measurements?.toolbarBorderBottomWidth).toBe('1px');
	expect(measurements?.toolbarBorderBottomColor).toBe('rgb(225, 229, 235)');
	expect(measurements?.rowBorderTopWidth).toBe('0px');
}

export async function expectDayAllDayRowCompact(page: Page): Promise<void> {
	const measurements = await page.evaluate(() => {
		const row = document.querySelector('.df-day-content-all-day-row');
		const label = document.querySelector('.df-all-day-label');
		if (!(row instanceof HTMLElement) || !(label instanceof HTMLElement)) return null;
		const rowRectangle = row.getBoundingClientRect();
		const labelRectangle = label.getBoundingClientRect();
		return {
			rowHeight: Math.round(rowRectangle.height),
			labelHeight: Math.round(labelRectangle.height),
			labelTopGap: Math.round(labelRectangle.top - rowRectangle.top)
		};
	});
	expect(measurements).not.toBeNull();
	expect(measurements?.rowHeight).toBeGreaterThanOrEqual(72);
	expect(measurements?.rowHeight).toBeLessThanOrEqual(76);
	expect(measurements?.labelHeight).toBeGreaterThanOrEqual(72);
	expect(measurements?.labelHeight).toBeLessThanOrEqual(76);
	expect(Math.abs(measurements?.labelTopGap ?? 0)).toBeLessThanOrEqual(1);
}

export async function expectDayAllDayRowEmptyCompact(page: Page): Promise<void> {
	const measurements = await page.evaluate(() => {
		const row = document.querySelector('.df-day-content-all-day-row');
		const label = document.querySelector('.df-all-day-label');
		if (!(row instanceof HTMLElement) || !(label instanceof HTMLElement)) return null;
		const rowRectangle = row.getBoundingClientRect();
		const labelRectangle = label.getBoundingClientRect();
		return {
			rowHeight: Math.round(rowRectangle.height),
			labelHeight: Math.round(labelRectangle.height),
			labelTopGap: Math.round(labelRectangle.top - rowRectangle.top)
		};
	});
	expect(measurements).not.toBeNull();
	expect(measurements?.rowHeight).toBeGreaterThanOrEqual(48);
	expect(measurements?.rowHeight).toBeLessThanOrEqual(52);
	expect(measurements?.labelHeight).toBeGreaterThanOrEqual(48);
	expect(measurements?.labelHeight).toBeLessThanOrEqual(52);
	expect(Math.abs(measurements?.labelTopGap ?? 0)).toBeLessThanOrEqual(1);
}

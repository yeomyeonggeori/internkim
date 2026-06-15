// 캘린더 embed interaction e2e의 assertion helper를 제공합니다.
import { expect, type Page } from '@playwright/test';
import { elementBox } from './calendar-embed-test-utils';

export async function expectRenderableCalendarEvent(page: Page, eventID: string, classSelector: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(
				({ eventID: targetEventID, classSelector: targetClassSelector }) => {
					const element = Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${targetEventID}"]${targetClassSelector}`)).find(
						(candidate) => {
							const rectangle = candidate.getBoundingClientRect();
							return rectangle.width > 0 && rectangle.height > 0;
						}
					);
					if (!element) return null;
					const rectangle = element.getBoundingClientRect();
					return {
						width: Math.round(rectangle.width),
						height: Math.round(rectangle.height)
					};
				},
				{ eventID, classSelector }
			)
		)
		.not.toBeNull();
}

export async function expectFirstVisibleTimeLabel(page: Page, expectedLabel: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => {
				const labels = Array.from(document.querySelectorAll<HTMLElement>('.df-time-label')).filter((label) => {
					const style = window.getComputedStyle(label);
					const rectangle = label.getBoundingClientRect();
					return style.display !== 'none' && style.visibility !== 'hidden' && rectangle.width > 0 && rectangle.height > 0 && rectangle.bottom > 0;
				});
				return labels[0]?.textContent?.trim() ?? '';
			})
		)
		.toBe(expectedLabel);
}

export async function expectElementHeightAtLeast(page: Page, selector: string, expectedMinimumHeight: number): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((targetSelector) => {
				const element = document.querySelector(targetSelector);
				if (!(element instanceof HTMLElement)) return 0;
				return Math.round(element.getBoundingClientRect().height);
			}, selector)
		)
		.toBeGreaterThanOrEqual(expectedMinimumHeight);
}

export async function expectTimelineScrollState(page: Page, scrollerSelector: string, pinnedSelector: string): Promise<void> {
	const initialPinnedBox = await elementBox(page, pinnedSelector);
	await page.evaluate((targetSelector) => {
		const scroller = document.querySelector(targetSelector);
		if (!(scroller instanceof HTMLElement)) throw new Error(`Missing timeline scroller: ${targetSelector}`);
		scroller.scrollTop = 260;
		scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
	}, scrollerSelector);

	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-scrolled/);
	await expect
		.poll(async () => elementBox(page, pinnedSelector))
		.toMatchObject({
			top: initialPinnedBox.top,
			bottom: initialPinnedBox.bottom
		});
}

export async function expectAllDayLabelAlignedWithTimeLabels(page: Page, allDayLabelSelector: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((selector) => {
				const textContentRectangle = (element: HTMLElement): DOMRect | null => {
					const textNode = Array.from(element.childNodes).find((node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim());
					if (!textNode) return null;
					const range = document.createRange();
					range.selectNodeContents(textNode);
					const rectangle = range.getBoundingClientRect();
					range.detach();
					return rectangle;
				};
				const allDayLabel = document.querySelector(selector);
				const firstTimeLabel = Array.from(document.querySelectorAll<HTMLElement>('.df-time-label')).find((label) => {
					const text = label.textContent?.trim() ?? '';
					const style = window.getComputedStyle(label);
					const rectangle = label.getBoundingClientRect();
					return text === '01:00' && style.display !== 'none' && rectangle.width > 0 && rectangle.height > 0;
				});
				if (!(allDayLabel instanceof HTMLElement) || !firstTimeLabel) return Number.POSITIVE_INFINITY;
				const allDayRectangle = allDayLabel.getBoundingClientRect();
				const timeTextRectangle = textContentRectangle(firstTimeLabel);
				const allDayTextRectangle = textContentRectangle(allDayLabel);
				const allDayRight = allDayTextRectangle?.right ?? allDayRectangle.right;
				const timeRight = timeTextRectangle?.right ?? firstTimeLabel.getBoundingClientRect().right;
				return Math.abs(allDayRight - timeRight);
			}, allDayLabelSelector)
		)
		.toBeLessThanOrEqual(2);
}

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

export async function expectMonthOverlayAlignedWithMonthStartRow(page: Page): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => {
				const overlay = Array.from(document.querySelectorAll<HTMLElement>('.month-scroll-overlay')).find((element) => {
					const rectangle = element.getBoundingClientRect();
					return rectangle.width > 0 && rectangle.height > 0;
				});
				if (!overlay) return Number.POSITIVE_INFINITY;
				const match = overlay.textContent?.trim().match(/(\d{4})년\s+(\d{1,2})월/);
				if (!match) return Number.POSITIVE_INFINITY;
				const [, year, month] = match;
				const dateKey = `${year}-${String(Number(month)).padStart(2, '0')}-01`;
				const monthStartCell = document.querySelector<HTMLElement>(`.df-month-day-cell[data-date="${dateKey}"]`);
				if (!monthStartCell) return Number.POSITIVE_INFINITY;
				const overlayRectangle = overlay.getBoundingClientRect();
				const cellRectangle = monthStartCell.getBoundingClientRect();
				if (overlayRectangle.top < cellRectangle.top) return cellRectangle.top - overlayRectangle.top;
				if (overlayRectangle.top > cellRectangle.bottom) return overlayRectangle.top - cellRectangle.bottom;
				return 0;
			})
		)
		.toBe(0);
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

export async function expectMonthWeekendCellsKeepGridLines(page: Page): Promise<void> {
	await page.waitForSelector('.df-month-day-cell[data-date="2026-06-07"]');
	await page.waitForSelector('.df-month-day-cell[data-date="2026-06-13"]');
	const styles = await page.evaluate(() => {
		const sunday = document.querySelector('.df-month-day-cell[data-date="2026-06-07"]');
		const saturday = document.querySelector('.df-month-day-cell[data-date="2026-06-13"]');
		if (!(sunday instanceof HTMLElement) || !(saturday instanceof HTMLElement)) return null;
		const sundayStyle = window.getComputedStyle(sunday);
		const saturdayStyle = window.getComputedStyle(saturday);
		return {
			saturdayBoxShadow: saturdayStyle.boxShadow,
			sundayBoxShadow: sundayStyle.boxShadow
		};
	});
	expect(styles).not.toBeNull();
	expect(styles?.saturdayBoxShadow).not.toBe('none');
	expect(styles?.sundayBoxShadow).not.toBe('none');
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

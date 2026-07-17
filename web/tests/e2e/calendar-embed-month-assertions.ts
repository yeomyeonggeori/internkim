import { expect, type Page } from '@playwright/test';

export async function expectMonthEventWithinDateCell(page: Page, eventID: string, dateKey: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(
				({ eventID: targetEventID, dateKey: targetDateKey }) => {
					const eventElement = Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${targetEventID}"].df-month-event`)).find(
						(candidate) => {
							const rectangle = candidate.getBoundingClientRect();
							return rectangle.width > 0 && rectangle.height > 0;
						}
					);
					const dayCellElement = document.querySelector<HTMLElement>(`.df-month-day-cell[data-date="${targetDateKey}"]`);
					if (!eventElement || !dayCellElement) {
						return {
							status: 'missing',
							isInsideCell: false
						};
					}
					const eventRectangle = eventElement.getBoundingClientRect();
					const cellRectangle = dayCellElement.getBoundingClientRect();
					const tolerance = 2;
					const leftGap = eventRectangle.left - cellRectangle.left;
					const rightGap = cellRectangle.right - eventRectangle.right;
					const widthGap = cellRectangle.width - eventRectangle.width;
					return {
						status: 'measured',
						eventWidth: Math.round(eventRectangle.width),
						cellWidth: Math.round(cellRectangle.width),
						leftGap: Math.round(leftGap),
						rightGap: Math.round(rightGap),
						widthGap: Math.round(widthGap),
						isInsideCell: leftGap >= -tolerance && rightGap >= -tolerance && widthGap >= -tolerance
					};
				},
				{ eventID, dateKey }
			)
		)
		.toMatchObject({
			status: 'measured',
			isInsideCell: true
		});
}

export async function expectMonthEventContentAligned(page: Page, eventID: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((targetEventID) => {
				const eventElement = Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${targetEventID}"].df-month-event`)).find(
					(candidate) => {
						const rectangle = candidate.getBoundingClientRect();
						return rectangle.width > 0 && rectangle.height > 0;
					}
				);
				const contentElement = eventElement?.querySelector<HTMLElement>(
					'.calendar-month-event-content, .df-month-segment-event, .df-content-slot'
				);
				if (!eventElement || !contentElement) {
					return {
						status: 'missing',
						isAligned: false
					};
				}
				const eventRectangle = eventElement.getBoundingClientRect();
				const contentRectangle = contentElement.getBoundingClientRect();
				const topGap = contentRectangle.top - eventRectangle.top;
				const bottomGap = eventRectangle.bottom - contentRectangle.bottom;
				const leftGap = contentRectangle.left - eventRectangle.left;
				const tolerance = 1;
				return {
					status: 'measured',
					topGap: Math.round(topGap),
					bottomGap: Math.round(bottomGap),
					leftGap: Math.round(leftGap),
					isAligned: Math.abs(topGap) <= tolerance && Math.abs(bottomGap) <= tolerance && leftGap >= 6 && leftGap <= 18
				};
			}, eventID)
		)
		.toMatchObject({
			status: 'measured',
			isAligned: true
		});
}

export async function expectMonthEventsShareBlockStyle(page: Page, firstEventID: string, secondEventID: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(
				({ firstID, secondID }) => {
					const firstEvent = visibleMonthEvent(firstID);
					const secondEvent = visibleMonthEvent(secondID);
					if (!firstEvent || !secondEvent) {
						return {
							status: 'missing',
							matches: false
						};
					}
					const firstStyle = blockStyle(firstEvent);
					const secondStyle = blockStyle(secondEvent);
					const firstBarStyle = window.getComputedStyle(firstEvent, '::before');
					const secondBarStyle = window.getComputedStyle(secondEvent, '::before');
					return {
						status: 'measured',
						firstStyle,
						secondStyle,
						firstBarBackground: firstBarStyle.backgroundColor,
						secondBarBackground: secondBarStyle.backgroundColor,
						matches:
							firstStyle.height === secondStyle.height &&
							firstStyle.minHeight === secondStyle.minHeight &&
							firstStyle.backgroundColor === secondStyle.backgroundColor &&
							firstStyle.borderRadius === secondStyle.borderRadius &&
							firstStyle.color === secondStyle.color &&
							firstStyle.fontSize === secondStyle.fontSize &&
							firstStyle.fontWeight === secondStyle.fontWeight &&
							firstStyle.lineHeight === secondStyle.lineHeight &&
							firstBarStyle.backgroundColor === secondBarStyle.backgroundColor
					};

					function visibleMonthEvent(eventID: string): HTMLElement | null {
						return (
							Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${eventID}"].df-month-event`)).find((element) => {
								const rectangle = element.getBoundingClientRect();
								return rectangle.width > 0 && rectangle.height > 0;
							}) ?? null
						);
					}

					function blockStyle(element: HTMLElement): Record<string, string> {
						const style = window.getComputedStyle(element);
						return {
							height: style.height,
							minHeight: style.minHeight,
							backgroundColor: style.backgroundColor,
							borderRadius: style.borderRadius,
							color: style.color,
							fontSize: style.fontSize,
							fontWeight: style.fontWeight,
							lineHeight: style.lineHeight
						};
					}
				},
				{ firstID: firstEventID, secondID: secondEventID }
			)
		)
		.toMatchObject({
			status: 'measured',
			matches: true
		});
}

export async function expectMonthTimedEventTitleOnly(page: Page, eventID: string, title: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(
				({ targetEventID, expectedTitle }) => {
					const eventElement = Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${targetEventID}"].df-month-event`)).find(
						(candidate) => {
							const rectangle = candidate.getBoundingClientRect();
							return rectangle.width > 0 && rectangle.height > 0;
						}
					);
					const titleElement = eventElement?.querySelector<HTMLElement>('.calendar-month-event-title') ?? null;
					const timeElement = eventElement?.querySelector<HTMLElement>('.calendar-month-event-time') ?? null;
					if (!eventElement || !titleElement) {
						return {
							status: 'missing',
							isArranged: false
						};
					}
					const eventRectangle = eventElement.getBoundingClientRect();
					const titleRectangle = titleElement.getBoundingClientRect();
					const tolerance = 1;
					return {
						status: 'measured',
						titleText: titleElement.textContent?.trim(),
						hasTimeElement: timeElement !== null,
						isArranged:
							titleElement.textContent?.trim() === expectedTitle &&
							timeElement === null &&
							titleRectangle.left >= eventRectangle.left &&
							titleRectangle.right <= eventRectangle.right &&
							Math.abs(titleRectangle.top - eventRectangle.top) <= tolerance
					};
				},
				{ targetEventID: eventID, expectedTitle: title }
			)
		)
		.toMatchObject({
			status: 'measured',
			isArranged: true
		});
}

export async function expectMonthEventFullBlockFocused(page: Page, eventID: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((targetEventID) => {
				const eventElement = Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${targetEventID}"].df-month-event`)).find(
					(element) => {
						const rectangle = element.getBoundingClientRect();
						return rectangle.width > 0 && rectangle.height > 0;
					}
				);
				const contentElement = eventElement?.querySelector<HTMLElement>('.calendar-month-event-content');
				if (!eventElement || !contentElement) {
					return {
						status: 'missing',
						isFocused: false,
						isFullBlock: false
					};
				}
				const eventRectangle = eventElement.getBoundingClientRect();
				const contentRectangle = contentElement.getBoundingClientRect();
				const eventStyle = window.getComputedStyle(eventElement);
				const contentStyle = window.getComputedStyle(contentElement);
				return {
					status: 'measured',
					isFocused: eventElement.classList.contains('internkim-calendar-event-focused'),
					eventBackground: eventStyle.backgroundColor,
					eventColor: eventStyle.color,
					contentBackground: contentStyle.backgroundColor,
					contentBoxShadow: contentStyle.boxShadow,
					eventWidth: Math.round(eventRectangle.width),
					contentWidth: Math.round(contentRectangle.width),
					isFullBlock:
						eventStyle.backgroundColor === 'rgb(37, 99, 235)' &&
						eventStyle.color === 'rgb(255, 255, 255)' &&
						contentRectangle.width < eventRectangle.width - 10 &&
						contentStyle.backgroundColor === 'rgba(0, 0, 0, 0)' &&
						contentStyle.boxShadow === 'none'
				};
			}, eventID)
		)
		.toMatchObject({
			status: 'measured',
			isFocused: true,
			isFullBlock: true
		});
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

export async function expectMonthWeekendCellsKeepGridLines(page: Page): Promise<void> {
	await page.waitForSelector('.df-month-day-cell[data-date="2026-06-07"]');
	await page.waitForSelector('.df-month-day-cell[data-date="2026-06-13"]');
	await expect
		.poll(async () =>
			page.evaluate(() => {
				const headerElements = Array.from(document.querySelectorAll<HTMLElement>('.df-week-grid > .df-day-label')).slice(0, 7);
				const dateKeys = ['2026-06-07', '2026-06-08', '2026-06-09', '2026-06-10', '2026-06-11', '2026-06-12', '2026-06-13'];
				const cellElements = dateKeys.map((dateKey) => document.querySelector<HTMLElement>(`.df-month-day-cell[data-date="${dateKey}"]`));
				const scrollerElement = document.querySelector<HTMLElement>('.df-month-view-virtual-scroller');
				if (headerElements.length !== 7 || cellElements.some((element) => element === null) || !scrollerElement) {
					return {
						status: 'missing',
						isAligned: false
					};
				}

				const rightEdgeDeltas = headerElements.map((headerElement, index) => {
					const cellElement = cellElements[index];
					if (!cellElement) return Number.POSITIVE_INFINITY;
					return Math.abs(headerElement.getBoundingClientRect().right - cellElement.getBoundingClientRect().right);
				});
				const weekendCells = [cellElements[0], cellElements[6]].filter((element): element is HTMLElement => element instanceof HTMLElement);
				const weekendStyles = weekendCells.map((element) => window.getComputedStyle(element));
				const maximumRightEdgeDelta = Math.max(...rightEdgeDeltas);
				const hasSingleGridLine = weekendStyles.every((style) => style.borderRightWidth === '1px' && style.boxShadow === 'none');
				const hasNoScrollbarGutter = scrollerElement.offsetWidth === scrollerElement.clientWidth;

				return {
					status: 'measured',
					maximumRightEdgeDelta: Math.round(maximumRightEdgeDelta * 100) / 100,
					hasSingleGridLine,
					hasNoScrollbarGutter,
					isAligned: maximumRightEdgeDelta <= 1 && hasSingleGridLine && hasNoScrollbarGutter
				};
			})
		)
		.toMatchObject({
			status: 'measured',
			isAligned: true
		});
}

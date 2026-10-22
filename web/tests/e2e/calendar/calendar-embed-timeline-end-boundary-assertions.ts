import { expect, type Page } from '@playwright/test';
import { expectVisibleDayBoundaryTick } from './calendar-embed-timeline-boundary-tick-assertions';

export async function expectTimelineEndsAt24(page: Page, scrollerSelector: string, boundaryLabelSelector: string): Promise<void> {
	await page.evaluate((targetScrollerSelector) => {
		const scroller = document.querySelector(targetScrollerSelector);
		if (!(scroller instanceof HTMLElement)) throw new Error(`Missing timeline scroller: ${targetScrollerSelector}`);
		scroller.scrollTop = scroller.scrollHeight;
		scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
	}, scrollerSelector);

	await expect
		.poll(async () =>
			page.evaluate(
				({ targetScrollerSelector, targetBoundaryLabelSelector }) => {
					const scroller = document.querySelector(targetScrollerSelector);
					const boundaryLabel = document.querySelector(targetBoundaryLabelSelector);
					if (!(scroller instanceof HTMLElement) || !(boundaryLabel instanceof HTMLElement)) {
						return {
							status: 'missing',
							isClippedAtBoundary: false
						};
					}
					const scrollerRectangle = scroller.getBoundingClientRect();
					const labelRectangle = boundaryLabel.getBoundingClientRect();
					const visibleLabels = Array.from(document.querySelectorAll<HTMLElement>('.df-time-label, .df-midnight-label'))
						.filter((label) => {
							const style = window.getComputedStyle(label);
							const rectangle = label.getBoundingClientRect();
							return (
								style.display !== 'none' &&
								style.visibility !== 'hidden' &&
								rectangle.width > 0 &&
								rectangle.height > 0 &&
								rectangle.bottom >= scrollerRectangle.top &&
								rectangle.top <= scrollerRectangle.bottom
							);
						});
					const bottomGap = scrollerRectangle.bottom - labelRectangle.bottom;
					const previousTimeLabel = visibleLabels
						.filter((label) => label !== boundaryLabel && label.textContent?.trim() !== '24:00')
						.at(-1);
					const boundaryStyle = window.getComputedStyle(boundaryLabel);
					const previousStyle = previousTimeLabel ? window.getComputedStyle(previousTimeLabel) : null;
					const previousTickElement = previousTimeLabel?.closest<HTMLElement>('.df-time-slot') ?? null;
					const boundaryTickContainer = boundaryLabel.closest<HTMLElement>(
						'.df-week-time-grid-boundary-tail, .df-day-content-grid-boundary-bottom'
					);
					const boundaryTickElement = boundaryLabel.closest('.df-day-content-grid-boundary-bottom')
						? document.querySelector<HTMLElement>('.df-time-column')
						: boundaryTickContainer;
					const previousTickStyle = previousTickElement ? window.getComputedStyle(previousTickElement, '::before') : null;
					const boundaryTickStyle = boundaryTickElement
						? window.getComputedStyle(
								boundaryTickElement,
								boundaryTickElement.matches('.df-time-column') ? '::after' : '::before'
							)
						: null;
					const tickRight = (element: HTMLElement, style: CSSStyleDeclaration): number | null => {
						const right = Number.parseFloat(style.right);
						if (Number.isNaN(right)) return null;
						return element.getBoundingClientRect().right - right;
					};
					const tickTop = (element: HTMLElement, style: CSSStyleDeclaration): number | null => {
						const top = Number.parseFloat(style.top);
						if (!Number.isNaN(top)) return element.getBoundingClientRect().top + top;
						const bottom = Number.parseFloat(style.bottom);
						if (!Number.isNaN(bottom)) return element.getBoundingClientRect().bottom - bottom;
						return null;
					};
					const previousTickRight =
						previousTickElement && previousTickStyle ? tickRight(previousTickElement, previousTickStyle) : null;
					const boundaryTickRight =
						boundaryTickElement && boundaryTickStyle ? tickRight(boundaryTickElement, boundaryTickStyle) : null;
					const boundaryTickTop = boundaryTickElement && boundaryTickStyle ? tickTop(boundaryTickElement, boundaryTickStyle) : null;
					const boundaryContainerTop = boundaryTickContainer?.getBoundingClientRect().top ?? null;
					const stage = document.querySelector<HTMLElement>('.calendar-stage');
					const stageBackgroundColor = stage ? window.getComputedStyle(stage).backgroundColor : null;
					const isDayBoundary = boundaryLabel.closest('.df-day-content-grid-boundary-bottom') !== null;
					const isWeekBoundary = boundaryLabel.closest('.df-week-time-grid-boundary-tail') !== null;
					const boundaryContainerStyle = boundaryTickContainer ? window.getComputedStyle(boundaryTickContainer) : null;
					const hasVisibleDayBoundaryRightGrid =
						isDayBoundary &&
						!!boundaryContainerStyle &&
						boundaryContainerStyle.borderRightStyle !== 'none' &&
						Number.parseFloat(boundaryContainerStyle.borderRightWidth) > 0;
					const timeColumn = document.querySelector<HTMLElement>('.df-time-column');
					const timeColumnMaskStyle = timeColumn ? window.getComputedStyle(timeColumn, '::before') : null;
					const timeColumnMaskTop =
						timeColumn && timeColumnMaskStyle ? tickTop(timeColumn, timeColumnMaskStyle) : null;
					const dayContent = document.querySelector<HTMLElement>('.df-day-content');
					const dayContentStyle = dayContent ? window.getComputedStyle(dayContent) : null;
					const dayContentRightGridStyle = dayContent ? window.getComputedStyle(dayContent, '::before') : null;
					const dayContentRightGridBottom =
						dayContent && dayContentRightGridStyle
							? dayContent.getBoundingClientRect().bottom - Number.parseFloat(dayContentRightGridStyle.bottom)
							: null;
					const dayContentRightEdgeMaskStyle = dayContent ? window.getComputedStyle(dayContent, '::after') : null;
					const dayContentRightEdgeMaskTop =
						dayContent && dayContentRightEdgeMaskStyle
							? dayContent.getBoundingClientRect().bottom - Number.parseFloat(dayContentRightEdgeMaskStyle.height)
							: null;
					const dayContentRows = document.querySelector<HTMLElement>('.df-day-content-grid-rows');
					const dayContentRowsStyle = dayContentRows ? window.getComputedStyle(dayContentRows) : null;
					const hasVisibleDayContentRowsRightGrid =
						isDayBoundary &&
						!!dayContentRowsStyle &&
						dayContentRowsStyle.borderRightStyle !== 'none' &&
						Number.parseFloat(dayContentRowsStyle.borderRightWidth) > 0;
					const dayContentRightMaskStyle = dayContentRows ? window.getComputedStyle(dayContentRows, '::after') : null;
					const dayContentRightMaskTop =
						dayContentRows && dayContentRightMaskStyle
							? dayContentRows.getBoundingClientRect().bottom - Number.parseFloat(dayContentRightMaskStyle.height)
							: null;
					const weekTimeGrid = document.querySelector<HTMLElement>('.df-week-time-grid-grid');
					const weekRightMaskStyle = weekTimeGrid ? window.getComputedStyle(weekTimeGrid, '::after') : null;
					const weekRightMaskTop =
						weekTimeGrid && weekRightMaskStyle
							? weekTimeGrid.getBoundingClientRect().bottom - Number.parseFloat(weekRightMaskStyle.height)
							: null;
					const bottomBoundaryCells = Array.from(
						document.querySelectorAll<HTMLElement>(
							'.df-time-grid-boundary-bottom.df-week-time-grid-boundary-row > .df-week-time-grid-boundary-cell'
						)
					);
					const hasVisibleBoundaryCellGrid = bottomBoundaryCells.some((cell) => {
						const style = window.getComputedStyle(cell);
						const hasBorder = style.borderRightStyle !== 'none' && Number.parseFloat(style.borderRightWidth) > 0;
						const hasBackground = style.backgroundColor !== 'rgba(0, 0, 0, 0)' && style.backgroundColor !== 'transparent';
						return hasBorder || hasBackground;
					});
					return {
						status: 'measured',
						boundaryLabel: boundaryLabel.textContent?.trim() ?? '',
						bottomGap: Math.round(bottomGap),
						hasVisibleBoundaryCellGrid,
						hasVisibleDayBoundaryRightGrid,
						hasVisibleDayContentRowsRightGrid,
						lastVisibleLabel: visibleLabels.at(-1)?.textContent?.trim() ?? '',
						isClippedAtBoundary: bottomGap >= -1 && bottomGap <= 4,
						matchesPreviousTick:
							!!previousTickStyle &&
							!!boundaryTickStyle &&
							boundaryTickStyle.width === previousTickStyle.width &&
							boundaryTickStyle.right === previousTickStyle.right &&
							boundaryTickStyle.borderTopColor === previousTickStyle.borderTopColor &&
							boundaryTickStyle.borderTopStyle === previousTickStyle.borderTopStyle &&
							boundaryTickStyle.borderTopWidth === previousTickStyle.borderTopWidth,
						matchesPreviousTickPosition:
							previousTickRight !== null &&
							boundaryTickRight !== null &&
							boundaryTickTop !== null &&
							boundaryContainerTop !== null &&
							Math.abs(boundaryTickRight - previousTickRight) <= 1 &&
							Math.abs(boundaryTickTop - boundaryContainerTop) <= 1,
						masksDayAxisBelowBoundary:
							!isDayBoundary ||
							(!!timeColumnMaskStyle &&
								timeColumnMaskTop !== null &&
								boundaryContainerTop !== null &&
								timeColumnMaskStyle.content !== 'none' &&
								timeColumnMaskStyle.backgroundColor === stageBackgroundColor &&
								Math.abs(timeColumnMaskTop - (boundaryContainerTop + 1)) <= 1),
						masksDayContentRightGridBelowBoundary:
							!isDayBoundary ||
							(!!dayContentStyle &&
								!!dayContentRightGridStyle &&
								dayContentStyle.borderRightStyle === 'none' &&
								Number.parseFloat(dayContentStyle.borderRightWidth) === 0 &&
								dayContentRightGridStyle.content === 'none') ||
							(!!dayContentStyle &&
								!!dayContentRightGridStyle &&
								dayContentRightGridBottom !== null &&
								boundaryContainerTop !== null &&
								dayContentStyle.borderRightStyle === 'none' &&
								Number.parseFloat(dayContentStyle.borderRightWidth) === 0 &&
								dayContentRightGridStyle.content !== 'none' &&
								dayContentRightGridStyle.backgroundColor === 'rgb(225, 229, 235)' &&
								dayContentRightGridStyle.right === '0px' &&
								Number.parseFloat(dayContentRightGridStyle.width) >= 1 &&
								Math.abs(dayContentRightGridBottom - boundaryContainerTop) <= 1),
						masksDayContentRightEdgeBelowBoundary:
							!isDayBoundary ||
							(!!dayContentRightEdgeMaskStyle &&
								dayContentRightEdgeMaskTop !== null &&
								boundaryContainerTop !== null &&
								dayContentRightEdgeMaskStyle.content !== 'none' &&
								dayContentRightEdgeMaskStyle.backgroundColor === stageBackgroundColor &&
								dayContentRightEdgeMaskStyle.right === '0px' &&
								Number.parseFloat(dayContentRightEdgeMaskStyle.width) >= 1 &&
								Math.abs(dayContentRightEdgeMaskTop - (boundaryContainerTop + 1)) <= 1),
						masksTimelineRightGridBelowBoundary:
							(!isDayBoundary && !isWeekBoundary) ||
							(isDayBoundary &&
								!!dayContentRightMaskStyle &&
								dayContentRightMaskTop !== null &&
								boundaryContainerTop !== null &&
								dayContentRightMaskStyle.content !== 'none' &&
								dayContentRightMaskStyle.backgroundColor === stageBackgroundColor &&
								dayContentRightMaskStyle.right === '0px' &&
								Number.parseFloat(dayContentRightMaskStyle.width) >= 1 &&
								Math.abs(dayContentRightMaskTop - (boundaryContainerTop + 1)) <= 1) ||
							(isWeekBoundary &&
								!!weekRightMaskStyle &&
								weekRightMaskTop !== null &&
								boundaryContainerTop !== null &&
								weekRightMaskStyle.content !== 'none' &&
								weekRightMaskStyle.backgroundColor === stageBackgroundColor &&
								weekRightMaskStyle.right === '0px' &&
								Number.parseFloat(weekRightMaskStyle.width) >= 1 &&
								Math.abs(weekRightMaskTop - (boundaryContainerTop + 1)) <= 1),
						matchesPreviousTimeLabel:
							!!previousStyle &&
							boundaryStyle.color === previousStyle.color &&
							(!isDayBoundary || boundaryStyle.backgroundColor === previousStyle.backgroundColor) &&
							boundaryStyle.fontFamily === previousStyle.fontFamily &&
							boundaryStyle.fontSize === previousStyle.fontSize &&
							boundaryStyle.fontWeight === previousStyle.fontWeight &&
							boundaryStyle.lineHeight === previousStyle.lineHeight
					};
				},
				{ targetScrollerSelector: scrollerSelector, targetBoundaryLabelSelector: boundaryLabelSelector }
			)
		)
		.toMatchObject({
			status: 'measured',
			boundaryLabel: '24:00',
			hasVisibleBoundaryCellGrid: false,
			hasVisibleDayBoundaryRightGrid: false,
			hasVisibleDayContentRowsRightGrid: false,
			lastVisibleLabel: '24:00',
			isClippedAtBoundary: true,
			masksDayContentRightGridBelowBoundary: true,
			masksDayContentRightEdgeBelowBoundary: true,
			masksDayAxisBelowBoundary: true,
			masksTimelineRightGridBelowBoundary: true,
			matchesPreviousTick: true,
			matchesPreviousTickPosition: true,
			matchesPreviousTimeLabel: true
		});

	await expectVisibleDayBoundaryTick(page, boundaryLabelSelector);
}

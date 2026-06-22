import { expect, type Page } from '@playwright/test';
import { elementBox } from './calendar-embed-test-utils';

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

export async function expectCalendarEventSelectedBlue(page: Page, selector: string): Promise<void> {
	const style = await page.evaluate((targetSelector) => {
		const element = document.querySelector(targetSelector);
		if (!(element instanceof HTMLElement)) return null;
		const computedStyle = window.getComputedStyle(element);
		const beforeStyle = window.getComputedStyle(element, '::before');
		return {
			backgroundColor: computedStyle.backgroundColor,
			beforeBackgroundColor: beforeStyle.backgroundColor,
			color: computedStyle.color
		};
	}, selector);
	expect(style).toEqual({
		backgroundColor: 'rgb(59, 130, 246)',
		beforeBackgroundColor: 'rgb(255, 255, 255)',
		color: 'rgb(255, 255, 255)'
	});
}

export async function expectCalendarEventSelectedOnPointerDown(page: Page, clickedSelector: string, linkedSelector: string): Promise<void> {
	const result = await page.evaluate(
		({ clickedTargetSelector, linkedTargetSelector }) => {
			const clickedElement = document.querySelector(clickedTargetSelector);
			const linkedElement = document.querySelector(linkedTargetSelector);
			if (!(clickedElement instanceof HTMLElement) || !(linkedElement instanceof HTMLElement)) return null;
			const rectangle = clickedElement.getBoundingClientRect();
			const clientX = rectangle.left + rectangle.width / 2;
			const clientY = rectangle.top + rectangle.height / 2;
			clickedElement.dispatchEvent(
				new PointerEvent('pointerdown', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: 41,
					clientX,
					clientY
				})
			);
			return {
				clickedClassName: clickedElement.className,
				linkedClassName: linkedElement.className
			};
		},
		{ clickedTargetSelector: clickedSelector, linkedTargetSelector: linkedSelector }
	);
	expect(result).not.toBeNull();
	expect(result?.clickedClassName).toContain('internkim-calendar-event-focused');
	expect(result?.linkedClassName).toContain('internkim-calendar-event-focused');
}

export async function expectCalendarEventTitleAndTime(page: Page, selector: string, title: string, time: string): Promise<void> {
	const event = page.locator(selector);
	await expect(event.locator('.calendar-event-title')).toHaveText(title);
	await expect(event.locator('.calendar-event-time')).toHaveText(time);
}

export async function expectTimelineEventsUseSeparateLanes(page: Page, firstSelector: string, secondSelector: string): Promise<void> {
	const measurements = await page.evaluate(
		({ firstTargetSelector, secondTargetSelector }) => {
			const firstElement = document.querySelector(firstTargetSelector);
			const secondElement = document.querySelector(secondTargetSelector);
			if (!(firstElement instanceof HTMLElement) || !(secondElement instanceof HTMLElement)) return null;
			const firstRectangle = firstElement.getBoundingClientRect();
			const secondRectangle = secondElement.getBoundingClientRect();
			const firstStyle = window.getComputedStyle(firstElement);
			const secondStyle = window.getComputedStyle(secondElement);
			return {
				firstBorderColor: firstStyle.borderColor,
				firstBorderWidth: firstStyle.borderWidth,
				firstClassName: firstElement.className,
				firstRight: Math.round(firstRectangle.right),
				firstWidth: Math.round(firstRectangle.width),
				horizontalGap: Math.round(secondRectangle.left - firstRectangle.right),
				secondBorderColor: secondStyle.borderColor,
				secondBorderWidth: secondStyle.borderWidth,
				secondClassName: secondElement.className,
				secondLeft: Math.round(secondRectangle.left),
				secondWidth: Math.round(secondRectangle.width),
				verticalOverlap: firstRectangle.top < secondRectangle.bottom && secondRectangle.top < firstRectangle.bottom
			};
		},
		{ firstTargetSelector: firstSelector, secondTargetSelector: secondSelector }
	);
	expect(measurements).not.toBeNull();
	expect(measurements?.firstClassName).toContain('calendar-timeline-lane-adjusted');
	expect(measurements?.secondClassName).toContain('calendar-timeline-lane-adjusted');
	expect(measurements?.verticalOverlap).toBe(true);
	expect(measurements?.firstWidth).toBeGreaterThan(20);
	expect(measurements?.secondWidth).toBeGreaterThan(20);
	expect(measurements?.horizontalGap).toBeGreaterThanOrEqual(-1);
	expect(measurements?.firstBorderColor).not.toBe('rgba(0, 0, 0, 0)');
	expect(measurements?.secondBorderColor).not.toBe('rgba(0, 0, 0, 0)');
	expect(measurements?.firstBorderWidth).toBe('1px');
	expect(measurements?.secondBorderWidth).toBe('1px');
}

export async function expectTimelineEventLayeredBehindLanes(page: Page, layerSelector: string, laneSelectors: string[]): Promise<void> {
	const measurements = await page.evaluate(
		({ layerTargetSelector, laneTargetSelectors }) => {
			const layerElement = document.querySelector(layerTargetSelector);
			const laneElements = laneTargetSelectors.map((selector) => document.querySelector(selector));
			if (!(layerElement instanceof HTMLElement) || laneElements.some((element) => !(element instanceof HTMLElement))) return null;
			const layerRectangle = layerElement.getBoundingClientRect();
			const layerStyle = window.getComputedStyle(layerElement);
			return {
				layerClassName: layerElement.className,
				layerWidth: Math.round(layerRectangle.width),
				layerZIndex: Number.parseInt(layerStyle.zIndex, 10),
				lanes: laneElements.map((laneElement) => {
					const element = laneElement as HTMLElement;
					const laneRectangle = element.getBoundingClientRect();
					const laneStyle = window.getComputedStyle(element);
					return {
						className: element.className,
						horizontalInsideLayer: layerRectangle.left <= laneRectangle.left + 1 && laneRectangle.right <= layerRectangle.right + 1,
						verticalOverlap: layerRectangle.top < laneRectangle.bottom && laneRectangle.top < layerRectangle.bottom,
						width: Math.round(laneRectangle.width),
						zIndex: Number.parseInt(laneStyle.zIndex, 10)
					};
				})
			};
		},
		{ layerTargetSelector: layerSelector, laneTargetSelectors: laneSelectors }
	);
	expect(measurements).not.toBeNull();
	expect(measurements?.layerClassName).toContain('calendar-timeline-layered');
	expect(measurements?.layerClassName).not.toContain('calendar-timeline-lane-adjusted');
	for (const lane of measurements?.lanes ?? []) {
		expect(lane.className).toContain('calendar-timeline-lane-adjusted');
		expect(lane.horizontalInsideLayer).toBe(true);
		expect(lane.verticalOverlap).toBe(true);
		expect(measurements?.layerWidth).toBeGreaterThan(lane.width);
		expect(lane.zIndex).toBeGreaterThan(measurements?.layerZIndex ?? 0);
	}
}

export async function expectTimelineEventNestedInsideLane(page: Page, nestedSelector: string, parentSelector: string): Promise<void> {
	const measurements = await page.evaluate(
		({ nestedTargetSelector, parentTargetSelector }) => {
			const nestedElement = document.querySelector(nestedTargetSelector);
			const parentElement = document.querySelector(parentTargetSelector);
			if (!(nestedElement instanceof HTMLElement) || !(parentElement instanceof HTMLElement)) return null;
			const nestedRectangle = nestedElement.getBoundingClientRect();
			const parentRectangle = parentElement.getBoundingClientRect();
			const nestedStyle = window.getComputedStyle(nestedElement);
			const parentStyle = window.getComputedStyle(parentElement);
			return {
				nestedClassName: nestedElement.className,
				nestedInsideParent:
					parentRectangle.left < nestedRectangle.left &&
					nestedRectangle.right < parentRectangle.right &&
					parentRectangle.top < nestedRectangle.top &&
					nestedRectangle.bottom < parentRectangle.bottom,
				nestedZIndex: Number.parseInt(nestedStyle.zIndex, 10),
				parentClassName: parentElement.className,
				parentZIndex: Number.parseInt(parentStyle.zIndex, 10)
			};
		},
		{ nestedTargetSelector: nestedSelector, parentTargetSelector: parentSelector }
	);
	expect(measurements).not.toBeNull();
	expect(measurements?.nestedClassName).toContain('calendar-timeline-layered');
	expect(measurements?.parentClassName).toContain('calendar-timeline-lane-adjusted');
	expect(measurements?.nestedInsideParent).toBe(true);
	expect(measurements?.nestedZIndex).toBeGreaterThan(measurements?.parentZIndex ?? 0);
}

export async function expectTimelinePreviewWithinGrid(page: Page, previewSelector: string, gridSelector: string): Promise<void> {
	const measurements = await page.evaluate(
		({ previewTargetSelector, gridTargetSelector }) => {
			const preview = document.querySelector(previewTargetSelector);
			const grid = document.querySelector(gridTargetSelector);
			if (!(preview instanceof HTMLElement) || !(grid instanceof HTMLElement)) return null;
			const previewRectangle = preview.getBoundingClientRect();
			const gridRectangle = grid.getBoundingClientRect();
			return {
				bottomOverflow: Math.round(previewRectangle.bottom - gridRectangle.bottom),
				leftOverflow: Math.round(gridRectangle.left - previewRectangle.left),
				rightOverflow: Math.round(previewRectangle.right - gridRectangle.right),
				topOverflow: Math.round(gridRectangle.top - previewRectangle.top)
			};
		},
		{ previewTargetSelector: previewSelector, gridTargetSelector: gridSelector }
	);
	expect(measurements).not.toBeNull();
	expect(measurements?.leftOverflow).toBeLessThanOrEqual(1);
	expect(measurements?.rightOverflow).toBeLessThanOrEqual(1);
	expect(measurements?.topOverflow).toBeLessThanOrEqual(1);
	expect(measurements?.bottomOverflow).toBeLessThanOrEqual(1);
}

export async function expectRightPanelEventCardsShareBlockStyle(page: Page, selectors: string[]): Promise<void> {
	const measurements = await page.evaluate((targetSelectors) => {
		const styles = targetSelectors.map((targetSelector) => {
			const element = document.querySelector(targetSelector);
			if (!(element instanceof HTMLElement)) return null;
			const elementStyle = window.getComputedStyle(element);
			const beforeStyle = window.getComputedStyle(element, '::before');
			const rectangle = element.getBoundingClientRect();
			return {
				backgroundColor: elementStyle.backgroundColor,
				beforeBackgroundColor: beforeStyle.backgroundColor,
				beforeBottom: beforeStyle.bottom,
				beforeHeight: beforeStyle.height,
				beforeLeft: beforeStyle.left,
				beforeTop: beforeStyle.top,
				beforeWidth: beforeStyle.width,
				borderRadius: elementStyle.borderRadius,
				boxShadow: elementStyle.boxShadow,
				height: `${Math.round(rectangle.height)}px`,
				overflow: elementStyle.overflow
			};
		});
		const rectangles = targetSelectors
			.map((targetSelector) => document.querySelector(targetSelector))
			.filter((element): element is HTMLElement => element instanceof HTMLElement)
			.map((element) => element.getBoundingClientRect())
			.sort((firstRectangle, secondRectangle) => firstRectangle.top - secondRectangle.top);
		const gaps = rectangles.slice(0, -1).map((rectangle, index) => `${Math.round(rectangles[index + 1].top - rectangle.bottom)}px`);
		return { styles, gaps };
	}, selectors);
	const expectedStyle = {
		backgroundColor: 'rgb(239, 246, 255)',
		beforeBackgroundColor: 'rgb(59, 130, 246)',
		beforeBottom: '3px',
		beforeHeight: '24px',
		beforeLeft: '2px',
		beforeTop: '3px',
		beforeWidth: '3px',
		borderRadius: '4px',
		boxShadow: 'none',
		height: '30px',
		overflow: 'hidden'
	};
	expect(measurements.styles).toEqual(selectors.map(() => expectedStyle));
	expect(measurements.gaps).toEqual(Array(Math.max(0, selectors.length - 1)).fill('8px'));
}

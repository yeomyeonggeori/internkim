import { expect, type Page } from '@playwright/test';

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
						verticalOverlap: layerRectangle.top < laneRectangle.bottom && layerRectangle.top < laneRectangle.bottom,
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

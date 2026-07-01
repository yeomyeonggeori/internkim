import { expect, type Page } from '@playwright/test';

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

export async function expectRightPanelEventContentCentered(page: Page, selector: string): Promise<void> {
	const measurement = await page.evaluate((targetSelector) => {
		const element = document.querySelector(targetSelector);
		const contentElement = element?.querySelector('.calendar-event-content');
		if (!(element instanceof HTMLElement) || !(contentElement instanceof HTMLElement)) return null;
		const elementRectangle = element.getBoundingClientRect();
		const contentRectangle = contentElement.getBoundingClientRect();
		const elementCenterY = elementRectangle.top + elementRectangle.height / 2;
		const contentCenterY = contentRectangle.top + contentRectangle.height / 2;
		return Math.round(Math.abs(elementCenterY - contentCenterY));
	}, selector);
	expect(measurement).not.toBeNull();
	expect(measurement).toBeLessThanOrEqual(1);
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

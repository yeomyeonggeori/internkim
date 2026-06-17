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

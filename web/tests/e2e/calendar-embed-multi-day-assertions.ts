import { expect, type Page } from '@playwright/test';

export async function expectMultiDayTimedProxy(page: Page, eventID: string, title: string, startTime: string): Promise<void> {
	const proxy = page.locator(`.calendar-multi-day-all-day-proxy[data-event-id="${eventID}::multi-day-proxy"]`);
	await expect(proxy).toHaveCount(1);
	await expect(proxy).toContainText(title);
	await expect(proxy.locator('.calendar-multi-day-all-day-proxy-start')).toHaveText(`${title} ${startTime}`);
	await expect(proxy.locator('.calendar-multi-day-all-day-proxy-end')).toHaveCount(0);
	await expect
		.poll(async () =>
			page.evaluate((targetEventID) => {
				const proxyElement = document.querySelector<HTMLElement>(
					`.calendar-multi-day-all-day-proxy[data-event-id="${targetEventID}::multi-day-proxy"]`
				);
				const rowElement = document.querySelector<HTMLElement>('.df-week-all-day-row-content, .df-all-day-row');
				if (!proxyElement || !rowElement) {
					return {
						status: 'missing',
						isLayered: false
					};
				}
				const proxyRectangle = proxyElement.getBoundingClientRect();
				const rowRectangle = rowElement.getBoundingClientRect();
				return {
					status: 'measured',
					height: Math.round(proxyRectangle.height),
					isLayered:
						proxyRectangle.height >= 16 &&
						proxyRectangle.top >= rowRectangle.top &&
						proxyRectangle.bottom <= rowRectangle.bottom + 1
				};
			}, eventID)
		)
		.toMatchObject({
			status: 'measured',
			isLayered: true
		});
}

export async function expectWeekAllDayEventsDoNotOverlap(page: Page, firstEventID: string, secondEventID: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(
				({ firstEventID: firstID, secondEventID: secondID }) => {
					const eventRectangle = (eventID: string): DOMRect | null => {
						const element = document.querySelector<HTMLElement>(`[data-event-id="${eventID}"]`);
						if (!element) return null;
						const rectangle = element.getBoundingClientRect();
						if (rectangle.width <= 0 || rectangle.height <= 0) return null;
						return rectangle;
					};
					const firstRectangle = eventRectangle(firstID);
					const secondRectangle = eventRectangle(secondID);
					if (!firstRectangle || !secondRectangle) {
						return {
							status: 'missing',
							overlaps: true
						};
					}
					const horizontalOverlap = Math.max(firstRectangle.left, secondRectangle.left) < Math.min(firstRectangle.right, secondRectangle.right);
					const verticalOverlap = Math.max(firstRectangle.top, secondRectangle.top) < Math.min(firstRectangle.bottom, secondRectangle.bottom);
					return {
						status: 'measured',
						overlaps: horizontalOverlap && verticalOverlap
					};
				},
				{ firstEventID, secondEventID }
			)
		)
		.toEqual({
			status: 'measured',
			overlaps: false
		});
}

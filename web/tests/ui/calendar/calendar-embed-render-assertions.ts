import { expect, type Page } from '@playwright/test';

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

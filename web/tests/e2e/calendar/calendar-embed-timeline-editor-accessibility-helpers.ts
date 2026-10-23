import { expect, type Locator, type Page } from '@playwright/test';

export async function prepareAccessibleParticipantSuggestions(page: Page): Promise<void> {
	await page.route('**/api/v1/tools/person_list/invoke', async (route) => {
		await route.fulfill({
			json: {
				result: {
					people: [{ personID: 'accessible-participant', name: '접근성 참여자', email: 'accessible-participant@example.com' }]
				}
			}
		});
	});
	await page.reload();
}

export function accessibleEventBlock(page: Page, eventID: string): Locator {
	return page.locator(`.calendar-stage [data-calendar-event-id="${eventID}"]`);
}

export function accessibleEventActivator(page: Page, eventID: string): Locator {
	return accessibleEventBlock(page, eventID);
}

export async function expectForcedColorFocusRing(locator: Locator): Promise<void> {
	await expect(locator).toBeFocused();
	const focusStyle = await locator.evaluate((element) => {
		const style = window.getComputedStyle(element);
		return {
			outlineColor: style.outlineColor,
			outlineStyle: style.outlineStyle,
			outlineWidth: style.outlineWidth
		};
	});
	expect(focusStyle.outlineStyle).toBe('solid');
	expect(Number.parseFloat(focusStyle.outlineWidth)).toBeGreaterThanOrEqual(2);
	expect(focusStyle.outlineColor).not.toBe('rgba(0, 0, 0, 0)');
}

export function colorContrastRatio(foreground: string, background: string): number {
	const foregroundLuminance = relativeLuminance(parseRGBColor(foreground));
	const backgroundLuminance = relativeLuminance(parseRGBColor(background));
	const lightLuminance = Math.max(foregroundLuminance, backgroundLuminance);
	const darkLuminance = Math.min(foregroundLuminance, backgroundLuminance);
	return (lightLuminance + 0.05) / (darkLuminance + 0.05);
}

function parseRGBColor(color: string): [number, number, number] {
	const channels = color.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)/);
	if (!channels) throw new Error(`Unsupported computed color: ${color}`);
	return [Number(channels[1]), Number(channels[2]), Number(channels[3])];
}

function relativeLuminance([red, green, blue]: [number, number, number]): number {
	const [linearRed, linearGreen, linearBlue] = [red, green, blue].map((channel) => {
		const normalizedChannel = channel / 255;
		return normalizedChannel <= 0.04045
			? normalizedChannel / 12.92
			: ((normalizedChannel + 0.055) / 1.055) ** 2.4;
	});
	return linearRed * 0.2126 + linearGreen * 0.7152 + linearBlue * 0.0722;
}

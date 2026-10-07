import { expect, type Locator } from '@playwright/test';

export type TouchArea = { width: number; height: number };

export async function touchAreaOf(control: Locator): Promise<TouchArea> {
	return control.evaluate((element) => {
		const box = element.getBoundingClientRect();
		const reach = getComputedStyle(element, '::after');
		const reachWidth = reach.content === 'none' ? 0 : parseFloat(reach.width) || 0;
		const reachHeight = reach.content === 'none' ? 0 : parseFloat(reach.height) || 0;
		return { width: Math.max(box.width, reachWidth), height: Math.max(box.height, reachHeight) };
	});
}

export async function expectTouchTarget(control: Locator): Promise<void> {
	await expect.poll(async () => (await touchAreaOf(control)).height).toBeGreaterThanOrEqual(44);
	await expect.poll(async () => (await touchAreaOf(control)).width).toBeGreaterThanOrEqual(44);
}

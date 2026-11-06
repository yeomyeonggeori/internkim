import type { Locator } from '@playwright/test';

export async function swipeSideways(start: Locator, fromClientX: number, toClientX: number) {
	await start.evaluate((element, path) => {
		const clientY = element.getBoundingClientRect().top + 4;
		const pointer = (type: string, clientX: number) =>
			element.dispatchEvent(new PointerEvent(type, { pointerId: 7, pointerType: 'touch', isPrimary: true, clientX, clientY, bubbles: true }));
		pointer('pointerdown', path.fromClientX);
		const steps = 6;
		for (let step = 1; step <= steps; step++) pointer('pointermove', path.fromClientX + ((path.toClientX - path.fromClientX) * step) / steps);
		pointer('pointerup', path.toClientX);
	}, { fromClientX, toClientX });
}

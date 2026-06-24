import { expect, type Page } from '@playwright/test';

type TickScanLine = {
	endX: number;
	startX: number;
	y: number;
};

type TickScanMeasurement =
	| {
			status: 'measured';
			boundary: TickScanLine;
			previous: TickScanLine;
	  }
	| {
			status: 'missing' | 'not-day';
	  };

export async function expectVisibleDayBoundaryTick(page: Page, boundaryLabelSelector: string): Promise<void> {
	const measurement = await page.evaluate((targetBoundaryLabelSelector): TickScanMeasurement => {
		const boundaryLabel = document.querySelector<HTMLElement>(targetBoundaryLabelSelector);
		if (!boundaryLabel?.closest('.df-day-content-grid-boundary-bottom')) return { status: 'not-day' };

		const timeColumn = document.querySelector<HTMLElement>('.df-time-column');
		const previousLabel = Array.from(document.querySelectorAll<HTMLElement>('.df-time-label')).find(
			(label) => label.textContent?.trim() === '23:00'
		);
		const previousSlot = previousLabel?.closest<HTMLElement>('.df-time-slot') ?? null;
		const boundaryContainer = boundaryLabel.closest<HTMLElement>('.df-day-content-grid-boundary-bottom');
		if (!timeColumn || !previousLabel || !previousSlot || !boundaryContainer) return { status: 'missing' };

		const tickWidth = Number.parseFloat(window.getComputedStyle(timeColumn, '::after').width);
		const timeColumnRectangle = timeColumn.getBoundingClientRect();
		const lineEndX = Math.floor(timeColumnRectangle.right) - 3;
		const lineFromTick = (y: number): TickScanLine => ({
			endX: lineEndX,
			startX: Math.ceil(timeColumnRectangle.right - tickWidth),
			y: Math.round(y)
		});

		return {
			status: 'measured',
			boundary: lineFromTick(boundaryContainer.getBoundingClientRect().top),
			previous: lineFromTick(previousSlot.getBoundingClientRect().top)
		};
	}, boundaryLabelSelector);

	if (measurement.status === 'not-day') return;
	expect(measurement.status).toBe('measured');
	if (measurement.status !== 'measured') return;

	const screenshot = await page.screenshot();
	const pixelCounts = await page.evaluate(
		async ({ imageBase64, targetMeasurement }) => {
			const response = await fetch(`data:image/png;base64,${imageBase64}`);
			const blob = await response.blob();
			const bitmap = await createImageBitmap(blob);
			const canvas = document.createElement('canvas');
			canvas.width = bitmap.width;
			canvas.height = bitmap.height;
			const context = canvas.getContext('2d');
			if (!context) throw new Error('Missing canvas context');
			context.drawImage(bitmap, 0, 0);
			const imageData = context.getImageData(0, 0, canvas.width, canvas.height);
			const countVisibleLinePixels = (line: TickScanLine): number => {
				const startX = Math.max(0, Math.min(canvas.width - 1, line.startX));
				const endX = Math.max(0, Math.min(canvas.width - 1, line.endX));
				if (endX < startX) return 0;
				let count = 0;
				for (let y = Math.max(0, line.y - 1); y <= Math.min(canvas.height - 1, line.y + 1); y += 1) {
					for (let x = startX; x <= endX; x += 1) {
						const offset = (y * canvas.width + x) * 4;
						const red = imageData.data[offset] ?? 255;
						const green = imageData.data[offset + 1] ?? 255;
						const blue = imageData.data[offset + 2] ?? 255;
						const alpha = imageData.data[offset + 3] ?? 0;
						if (alpha > 0 && red >= 200 && red < 245 && green >= 200 && green < 245 && blue >= 200 && blue < 245) {
							count += 1;
						}
					}
				}
				return count;
			};

			return {
				boundary: countVisibleLinePixels(targetMeasurement.boundary),
				previous: countVisibleLinePixels(targetMeasurement.previous)
			};
		},
		{ imageBase64: screenshot.toString('base64'), targetMeasurement: measurement }
	);
	expect(pixelCounts.previous).toBeGreaterThan(0);
	expect(pixelCounts.boundary).toBeGreaterThanOrEqual(pixelCounts.previous);
}

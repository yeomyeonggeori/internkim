type RGB = [number, number, number];

export type ContrastGrade = 'AAA' | 'AA' | 'AA large' | 'fail';

let paintingContext: CanvasRenderingContext2D | null = null;

function canvasContext(): CanvasRenderingContext2D | null {
	if (paintingContext) return paintingContext;
	const canvas = document.createElement('canvas');
	canvas.width = 1;
	canvas.height = 1;
	paintingContext = canvas.getContext('2d', { willReadFrequently: true });
	return paintingContext;
}

export function rgbOf(color: string): RGB | undefined {
	const context = canvasContext();
	if (!context) return undefined;
	context.clearRect(0, 0, 1, 1);
	context.fillStyle = '#000';
	context.fillStyle = color;
	context.fillRect(0, 0, 1, 1);
	const [red, green, blue] = context.getImageData(0, 0, 1, 1).data;
	return [red, green, blue];
}

function channelLuminance(channel: number): number {
	const value = channel / 255;
	return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
}

function relativeLuminance([red, green, blue]: RGB): number {
	return 0.2126 * channelLuminance(red) + 0.7152 * channelLuminance(green) + 0.0722 * channelLuminance(blue);
}

export function contrastRatio(foreground: string, background: string): number | undefined {
	const foregroundRGB = rgbOf(foreground);
	const backgroundRGB = rgbOf(background);
	if (!foregroundRGB || !backgroundRGB) return undefined;
	const [lighter, darker] = [relativeLuminance(foregroundRGB), relativeLuminance(backgroundRGB)].sort((a, b) => b - a);
	return (lighter + 0.05) / (darker + 0.05);
}

export function contrastGrade(ratio: number): ContrastGrade {
	if (ratio >= 7) return 'AAA';
	if (ratio >= 4.5) return 'AA';
	if (ratio >= 3) return 'AA large';
	return 'fail';
}

export function hexOf(color: string): string {
	const rgb = rgbOf(color);
	if (!rgb) return '';
	return `#${rgb.map((channel) => channel.toString(16).padStart(2, '0')).join('')}`.toUpperCase();
}

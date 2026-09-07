export type HSLColor = {
	hue: number;
	saturation: number;
	lightness: number;
};

export function hexToHSL(hex: string): HSLColor {
	const red = parseInt(hex.slice(1, 3), 16) / 255;
	const green = parseInt(hex.slice(3, 5), 16) / 255;
	const blue = parseInt(hex.slice(5, 7), 16) / 255;
	const highest = Math.max(red, green, blue);
	const lowest = Math.min(red, green, blue);
	const span = highest - lowest;
	const lightness = (highest + lowest) / 2;
	if (span === 0) return { hue: 0, saturation: 0, lightness: Math.round(lightness * 100) };
	const saturation = span / (1 - Math.abs(2 * lightness - 1));
	return {
		hue: Math.round(hueOf(red, green, blue, highest, span)),
		saturation: Math.round(saturation * 100),
		lightness: Math.round(lightness * 100)
	};
}

export function hslToHex({ hue, saturation, lightness }: HSLColor): string {
	const chroma = (1 - Math.abs((2 * lightness) / 100 - 1)) * (saturation / 100);
	const secondary = chroma * (1 - Math.abs(((hue / 60) % 2) - 1));
	const offset = lightness / 100 - chroma / 2;
	const [red, green, blue] = channelsOf(((hue % 360) + 360) % 360, chroma, secondary);
	return `#${[red, green, blue].map((channel) => hexPair(channel + offset)).join('')}`;
}

export function averageSaturation(colors: readonly string[]): number {
	const saturations = colors.map((color) => hexToHSL(color).saturation);
	return Math.round(saturations.reduce((total, saturation) => total + saturation, 0) / saturations.length);
}

function hueOf(red: number, green: number, blue: number, highest: number, span: number): number {
	if (highest === red) return (((green - blue) / span) % 6) * 60 + (green < blue ? 360 : 0);
	if (highest === green) return ((blue - red) / span + 2) * 60;
	return ((red - green) / span + 4) * 60;
}

function channelsOf(hue: number, chroma: number, secondary: number): [number, number, number] {
	if (hue < 60) return [chroma, secondary, 0];
	if (hue < 120) return [secondary, chroma, 0];
	if (hue < 180) return [0, chroma, secondary];
	if (hue < 240) return [0, secondary, chroma];
	if (hue < 300) return [secondary, 0, chroma];
	return [chroma, 0, secondary];
}

function hexPair(channel: number): string {
	return Math.round(Math.min(Math.max(channel, 0), 1) * 255)
		.toString(16)
		.padStart(2, '0');
}

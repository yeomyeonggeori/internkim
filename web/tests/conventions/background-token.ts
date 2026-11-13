export type ThemeSelector = ':root' | '.dark';

export function backgroundTokenOf(css: string, selector: ThemeSelector): string {
	const selectorList = new RegExp(`${selector.replace('.', '\\.')}[^{]*\\{`);
	const block = css.slice(css.search(selectorList));
	const match = block.match(/--background:\s*hsl\(([\d.]+)\s+([\d.]+)%\s+([\d.]+)%\)/);
	if (!match) throw new Error(`no --background token under ${selector}`);
	return hslToHex(Number(match[1]), Number(match[2]) / 100, Number(match[3]) / 100);
}

function hslToHex(hue: number, saturation: number, lightness: number): string {
	const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation;
	const sector = (hue / 60) % 2;
	const secondary = chroma * (1 - Math.abs(sector - 1));
	const offset = lightness - chroma / 2;
	const sectors: [number, number, number][] = [
		[chroma, secondary, 0],
		[secondary, chroma, 0],
		[0, chroma, secondary],
		[0, secondary, chroma],
		[secondary, 0, chroma],
		[chroma, 0, secondary]
	];
	const [red, green, blue] = sectors[Math.floor(hue / 60) % 6];
	return [red, green, blue]
		.map((channel) => Math.round((channel + offset) * 255).toString(16).padStart(2, '0'))
		.join('')
		.toUpperCase();
}

export const colorPickerPalette = [
	'#db3333',
	'#c64d2f',
	'#b65c20',
	'#996a24',
	'#887320',
	'#78781c',
	'#697c1d',
	'#59801e',
	'#45801e',
	'#33841f',
	'#1f841f',
	'#1f8433',
	'#1f8447',
	'#1f845c',
	'#1e806c',
	'#1e8080',
	'#237e95',
	'#207ab6',
	'#216fe4',
	'#4969e9',
	'#4949e9',
	'#6949e9',
	'#8949e9',
	'#a540e7',
	'#b43dd1',
	'#c22ec2',
	'#ca2fab',
	'#cf308f',
	'#d03975',
	'#d13d5b',
	'#991b1b',
	'#a16207',
	'#166534',
	'#115e59',
	'#1e40af',
	'#475569',
] as const;

export const fallbackPickerColor = '#64748b';

export function normalizeColor(value: string): string {
	const trimmedValue = value.trim().toLowerCase();
	if (/^#[0-9a-f]{6}$/.test(trimmedValue)) return trimmedValue;
	if (/^#[0-9a-f]{3}$/.test(trimmedValue)) {
		const [, red, green, blue] = trimmedValue;
		return `#${red}${red}${green}${green}${blue}${blue}`;
	}
	return fallbackPickerColor;
}

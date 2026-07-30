export const colorPickerPalette = [
	'#2563eb',
	'#1d4ed8',
	'#1e40af',
	'#0369a1',
	'#0e7490',
	'#115e59',
	'#0f766e',
	'#047857',
	'#15803d',
	'#166534',
	'#4d7c0f',
	'#14532d',
	'#a16207',
	'#713f12',
	'#b45309',
	'#c2410c',
	'#422006',
	'#78716c',
	'#dc2626',
	'#b91c1c',
	'#991b1b',
	'#7f1d1d',
	'#be123c',
	'#9f1239',
	'#db2777',
	'#be185d',
	'#831843',
	'#a21caf',
	'#7e22ce',
	'#7c3aed',
	'#6d28d9',
	'#4f46e5',
	'#4338ca',
	'#3730a3',
	'#475569',
	'#3f3f46',
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

export const colorPickerPalette = [
	'#2563eb',
	'#0891b2',
	'#0f766e',
	'#16a34a',
	'#ca8a04',
	'#ea580c',
	'#dc2626',
	'#be123c',
	'#db2777',
	'#7c3aed',
	'#4f46e5',
	'#64748b'
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

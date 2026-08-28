export const taskTypeColors = [
	'#86d79b',
	'#f5cf6b',
	'#f2a6a6',
	'#93b7f3',
	'#62c9d4',
	'#fb8f45',
	'#5bbf7a',
	'#f3bd3e',
	'#ef6b68',
	'#6d9df6',
	'#3fb8b1',
	'#f97316',
	'#34a853',
	'#f4b400',
	'#ea4335',
	'#4285f4',
	'#8b5cf6',
	'#ec4899',
	'#64748b',
	'#a16207',
	'#0ea5e9',
	'#84cc16',
	'#d946ef',
	'#14b8a6',
	'#f43f5e',
	'#8b9467',
	'#7c3aed',
	'#0891b2',
	'#ca8a04',
	'#be123c',
	'#4f46e5'
] as const;

export const taskProjectColors = ['#2563eb', '#0f766e', '#7c3aed', '#64748b', '#ea580c', '#0891b2'] as const;

export function taskTypeColor(index: number): string {
	return colorAt(taskTypeColors, index);
}

export function taskProjectColor(index: number): string {
	return colorAt(taskProjectColors, index);
}

function colorAt(colors: readonly string[], index: number): string {
	return colors[Math.abs(index) % colors.length] ?? '#64748b';
}

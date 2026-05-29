import type { AttendanceLocation } from '../attendance-context.svelte';

export type CellTone = 'default' | 'remote' | 'office' | 'empty' | 'weekend';

export const HEAT_LEVEL_CLASSES = {
	0: 'bg-muted/50',
	1: 'bg-success/15',
	2: 'bg-success/40',
	3: 'bg-success/80 text-success-foreground',
} as const;

export const STATUS_TONE = {
	working: 'text-success',
	finished: 'text-muted-foreground',
	absent: 'text-destructive',
	weekend: 'text-muted-foreground',
	upcoming: 'text-muted-foreground',
} as const;

export function locationCellClass(location?: AttendanceLocation | null): string {
	if (!location) return 'bg-success/15';
	if (location.isDefault) return 'bg-success/15';
	const name = location.name.toLowerCase();
	if (name.includes('재택') || name.includes('remote') || name.includes('home')) {
		return 'bg-info/15';
	}
	return 'bg-secondary';
}

export function locationChipStyle(location?: AttendanceLocation | null): string {
	if (!location?.color) return '';
	return `background-color: ${location.color}20; color: ${location.color};`;
}

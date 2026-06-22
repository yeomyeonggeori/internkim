import type { AttendanceLocation } from '../attendance-context.svelte';

export type CellTone = 'default' | 'remote' | 'office' | 'empty' | 'weekend';
export type AbsenceDisplayTone = 'leave' | 'work' | 'other';

export const HEAT_LEVEL_CLASSES = {
	0: 'bg-muted/50',
	1: 'bg-success/15',
	2: 'bg-success/40',
	3: 'bg-success/80 text-success-foreground',
} as const;

export const STATUS_TONE = {
	working: 'text-success',
	finished: 'text-muted-foreground',
	absence: 'text-info',
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

export function locationBadgeClass(locationName?: string, locationID?: string, isCurrent = false): string {
	const tone = locationTone(locationName, locationID);
	if (!isCurrent) return 'border-border bg-background text-muted-foreground';
	if (tone === 'remote') return 'border-info/30 bg-info/10 text-info';
	if (tone === 'field') return 'border-warning/40 bg-warning-subtle text-warning-subtle-foreground';
	return 'border-success/30 bg-success/10 text-success';
}

export function locationDotClass(locationName?: string, locationID?: string): string {
	const tone = locationTone(locationName, locationID);
	if (tone === 'remote') return 'bg-info';
	if (tone === 'field') return 'bg-warning';
	return 'bg-success';
}

export function absenceDisplayClass(tone: AbsenceDisplayTone, hasLeadingBorder = false): string {
	const borderClass = absenceBorderClass(tone, hasLeadingBorder);
	if (tone === 'work') return `${borderClass} bg-warning-subtle text-warning-subtle-foreground`;
	if (tone === 'other') return `${borderClass} bg-muted text-muted-foreground`;
	return `${borderClass} bg-[color-mix(in_oklab,var(--color-info)_12%,var(--color-background))] text-info`;
}

function absenceBorderClass(tone: AbsenceDisplayTone, hasLeadingBorder: boolean): string {
	if (!hasLeadingBorder) return '';
	if (tone === 'work') return 'border-l-warning';
	if (tone === 'other') return 'border-l-muted-foreground/40';
	return 'border-l-info';
}

function locationTone(locationName?: string, locationID?: string): 'office' | 'remote' | 'field' {
	const value = `${locationID ?? ''} ${locationName ?? ''}`.toLowerCase();
	if (value.includes('재택') || value.includes('remote') || value.includes('home')) return 'remote';
	if (
		value.includes('외부') ||
		value.includes('outside') ||
		value.includes('field') ||
		value.includes('bss')
	) {
		return 'field';
	}
	return 'office';
}

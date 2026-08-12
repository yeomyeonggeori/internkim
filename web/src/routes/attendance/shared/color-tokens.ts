import { dayOffColor } from '$lib/calendar/day-off-color';

export type AbsenceDisplayTone = 'leave' | 'other';

export const HEAT_LEVEL_CLASSES = {
	0: 'bg-muted/50',
	1: 'bg-success/15',
	2: 'bg-success/40',
	3: 'bg-success/80 text-success-foreground',
} as const;

export const STATUS_TONE = {
	working: 'text-success',
	finished: 'text-muted-foreground',
	absence: 'text-destructive',
	absent: 'text-destructive',
	weekend: 'text-muted-foreground',
	upcoming: 'text-muted-foreground',
} as const;

export function absenceDisplayClass(tone: AbsenceDisplayTone, hasLeadingBorder = false): string {
	const borderClass = absenceBorderClass(tone, hasLeadingBorder);
	if (tone === 'other') return `${borderClass} bg-muted text-muted-foreground`;
	return `${borderClass} bg-[color-mix(in_oklab,var(--color-destructive)_12%,var(--color-background))] text-destructive`;
}

function absenceBorderClass(tone: AbsenceDisplayTone, hasLeadingBorder: boolean): string {
	if (!hasLeadingBorder) return '';
	if (tone === 'other') return 'border-l-muted-foreground/40';
	return 'border-l-destructive';
}

export const ABSENCE_TONE = {
	leave: { label: 'text-destructive', meter: 'bg-destructive/30', bar: dayOffColor },
	other: { label: 'text-muted-foreground', meter: 'bg-muted-foreground/30', bar: 'var(--color-muted-foreground)' }
} as const satisfies Record<AbsenceDisplayTone, { label: string; meter: string; bar: string }>;

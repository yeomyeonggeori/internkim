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

import { formatMonthScrollOverlayText } from './calendar-embed-view-helpers';
import type { CalendarMonthScrollLabel } from './calendar-wheel-navigation';

export type VisibleMonthScrollLabel = CalendarMonthScrollLabel & {
	text: string;
	isFading: boolean;
};

export function visibleMonthScrollOverlayLabels(
	labels: CalendarMonthScrollLabel[],
	localeCode: string
): VisibleMonthScrollLabel[] {
	return labels.map((label) => ({
		...label,
		text: formatMonthScrollOverlayText(label.date, localeCode),
		isFading: false
	}));
}

export function fadingMonthScrollOverlayLabels(labels: VisibleMonthScrollLabel[]): VisibleMonthScrollLabel[] {
	return labels.map((label) => ({ ...label, isFading: true }));
}

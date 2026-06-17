import type { CalendarMonthScrollLabel } from './calendar-wheel-navigation';
import {
	fadingMonthScrollOverlayLabels,
	visibleMonthScrollOverlayLabels,
	type VisibleMonthScrollLabel
} from './calendar-month-scroll-overlay-state';

type CalendarPageScrollOverlayContext = {
	getLocaleCode: () => string;
	getMonthScrollOverlayLabels: () => VisibleMonthScrollLabel[];
	setMonthScrollOverlayLabels: (labels: VisibleMonthScrollLabel[]) => void;
};

export type CalendarPageScrollOverlayActions = {
	clearMonthScrollOverlays: () => void;
	showMonthScrollOverlays: (labels: CalendarMonthScrollLabel[]) => void;
};

export function createCalendarPageScrollOverlays(
	context: CalendarPageScrollOverlayContext
): CalendarPageScrollOverlayActions {
	let monthScrollOverlayTimer: number | null = null;

	function showMonthScrollOverlays(labels: CalendarMonthScrollLabel[]): void {
		context.setMonthScrollOverlayLabels(visibleMonthScrollOverlayLabels(labels, context.getLocaleCode()));
		if (monthScrollOverlayTimer) window.clearTimeout(monthScrollOverlayTimer);
		monthScrollOverlayTimer = null;
	}

	function clearMonthScrollOverlays(): void {
		if (monthScrollOverlayTimer) window.clearTimeout(monthScrollOverlayTimer);
		const currentLabels = context.getMonthScrollOverlayLabels();
		if (currentLabels.length === 0) {
			monthScrollOverlayTimer = null;
			return;
		}
		context.setMonthScrollOverlayLabels(fadingMonthScrollOverlayLabels(currentLabels));
		monthScrollOverlayTimer = window.setTimeout(() => {
			context.setMonthScrollOverlayLabels([]);
			monthScrollOverlayTimer = null;
		}, 180);
	}

	return {
		clearMonthScrollOverlays,
		showMonthScrollOverlays
	};
}

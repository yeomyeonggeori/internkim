import type { CalendarViewType } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import { anchorFromElement } from './calendar-draft-popover-anchor';
import { dateKey, type DraftPopoverAnchor } from './calendar-draft-popover-state';

export type CalendarAllDayCellActionOptions = {
	stageElement: HTMLElement;
	currentView: () => CalendarViewType;
	currentDate: () => Date;
	createAllDayEvent: (dateKey: string, anchor: DraftPopoverAnchor) => void;
	createMobileAllDayEvent: (dateKey: string) => void;
	isMobileEventEditor: () => boolean;
};

export function installCalendarAllDayCellAction(options: CalendarAllDayCellActionOptions): () => void {
	const handleClick = (event: MouseEvent) => {
		const target = allDayTarget(options, event.target);
		if (!target) return;
		const targetDateKey = allDayTargetDateKey(options, target);
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		if (options.isMobileEventEditor()) {
			options.createMobileAllDayEvent(targetDateKey);
			return;
		}
		const anchor = anchorFromElement(target);
		if (!anchor) return;
		options.createAllDayEvent(targetDateKey, anchor);
	};

	options.stageElement.addEventListener('click', handleClick, true);

	return () => {
		options.stageElement.removeEventListener('click', handleClick, true);
	};
}

function allDayTarget(options: CalendarAllDayCellActionOptions, target: EventTarget | null): HTMLElement | null {
	if (!(target instanceof Element)) return null;
	if (target.closest('.df-event, .calendar-draft-popover, .df-event-detail-panel, .df-dialog-container')) return null;
	if (options.currentView() === ViewType.DAY) return target.closest<HTMLElement>('.df-day-content-all-day-lane');
	if (options.currentView() === ViewType.WEEK) return target.closest<HTMLElement>('.df-week-all-day-cell');
	return null;
}

function allDayTargetDateKey(options: CalendarAllDayCellActionOptions, target: HTMLElement): string {
	if (options.currentView() === ViewType.WEEK) {
		const cellIndex = weekAllDayCellIndex(options.stageElement, target);
		const weekStartDate = weekStart(options.currentDate());
		return dateKey(new Date(weekStartDate.getFullYear(), weekStartDate.getMonth(), weekStartDate.getDate() + cellIndex));
	}
	return dateKey(options.currentDate());
}

function weekAllDayCellIndex(stageElement: HTMLElement, target: HTMLElement): number {
	const cells = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-week-all-day-cell'));
	return Math.max(0, cells.indexOf(target));
}

function weekStart(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate() - date.getDay());
}

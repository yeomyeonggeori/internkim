import { calendarEventElementsByID, isVisibleCalendarEventElement } from './calendar-event-elements';
import { shouldDeferCalendarDialogEscape } from './calendar-dialog-escape';

export type CalendarDraftPopoverFocusOptions = {
	closePopover: () => void | Promise<void>;
	eventID: string;
	isReady: boolean;
	originElement: HTMLElement | null;
	selectInitialFocus: boolean;
	stageElement: HTMLElement | null;
};

const initialFocusSelector = '[data-draft-popover-initial-focus]';
const eventFocusTargetSelector = '.calendar-dayflow-event-activator, button, [role="button"][tabindex]';
const focusRestorationRetryDelays = [50, 150, 300, 600, 1_100] as const;
let focusRestorationFrame: number | null = null;
let focusRestorationTimeouts: number[] = [];

export function installCalendarDraftPopoverFocus(
	popoverElement: HTMLElement,
	initialOptions: CalendarDraftPopoverFocusOptions
): { update: (options: CalendarDraftPopoverFocusOptions) => void; destroy: () => void } {
	let options = initialOptions;
	let focusFrame: number | null = null;
	clearCalendarDraftPopoverFocusRestoration();

	const scheduleInitialFocus = (): void => {
		if (focusFrame !== null) cancelAnimationFrame(focusFrame);
		if (!options.isReady) return;
		focusFrame = requestAnimationFrame(() => {
			focusFrame = null;
			if (popoverElement.contains(document.activeElement)) return;
			const focusElement = popoverElement.querySelector<HTMLElement>(initialFocusSelector);
			if (!focusElement) return;
			focusElement.focus({ preventScroll: true });
			if (options.selectInitialFocus && focusElement instanceof HTMLInputElement) focusElement.select();
		});
	};
	const handleKeydown = (event: KeyboardEvent): void => {
		if (event.key !== 'Escape') return;
		if (shouldDeferCalendarDialogEscape(event.target)) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		void options.closePopover();
	};

	popoverElement.addEventListener('keydown', handleKeydown, true);
	scheduleInitialFocus();
	return {
		update: (nextOptions) => {
			const shouldScheduleInitialFocus =
				nextOptions.eventID !== options.eventID || (!options.isReady && nextOptions.isReady);
			options = nextOptions;
			if (shouldScheduleInitialFocus) scheduleInitialFocus();
		},
		destroy: () => {
			popoverElement.removeEventListener('keydown', handleKeydown, true);
			if (focusFrame !== null) cancelAnimationFrame(focusFrame);
			scheduleCalendarDraftPopoverFocusRestoration(popoverElement, options);
		}
	};
}

function scheduleCalendarDraftPopoverFocusRestoration(
	popoverElement: HTMLElement,
	options: CalendarDraftPopoverFocusOptions
): void {
	clearCalendarDraftPopoverFocusRestoration();
	let restoredElement: HTMLElement | null = null;
	const restoreFocus = (): boolean => {
		if (document.querySelector('.calendar-draft-popover')) return true;
		if (hasMeaningfulFocusOutsidePopover(popoverElement) && document.activeElement !== restoredElement) {
			clearCalendarDraftPopoverFocusRestoration();
			return false;
		}
		if (
			restoredElement?.isConnected &&
			document.activeElement === restoredElement &&
			restoredElement !== options.stageElement
		) {
			return true;
		}
		restoredElement = restoredCalendarDraftPopoverFocus(options) ?? restoredElement;
		return true;
	};
	focusRestorationFrame = requestAnimationFrame(() => {
		focusRestorationFrame = null;
		if (!restoreFocus()) return;
		focusRestorationTimeouts = focusRestorationRetryDelays.map((delay) => window.setTimeout(restoreFocus, delay));
	});
}

function clearCalendarDraftPopoverFocusRestoration(): void {
	if (focusRestorationFrame !== null) cancelAnimationFrame(focusRestorationFrame);
	focusRestorationFrame = null;
	for (const timeout of focusRestorationTimeouts) window.clearTimeout(timeout);
	focusRestorationTimeouts = [];
}

function restoredCalendarDraftPopoverFocus(options: CalendarDraftPopoverFocusOptions): HTMLElement | null {
	if (focusElement(options.originElement)) return options.originElement;
	const eventFocusTarget = visibleEventFocusTarget(options.stageElement, options.eventID);
	if (focusElement(eventFocusTarget)) return eventFocusTarget;
	return focusElement(options.stageElement) ? options.stageElement : null;
}

function hasMeaningfulFocusOutsidePopover(popoverElement: HTMLElement): boolean {
	const activeElement = document.activeElement;
	if (!(activeElement instanceof HTMLElement) || !activeElement.isConnected) return false;
	if (activeElement === document.body || activeElement === document.documentElement) return false;
	return !popoverElement.contains(activeElement);
}

function visibleEventFocusTarget(stageElement: HTMLElement | null, eventID: string): HTMLElement | null {
	const eventElements = calendarEventElementsByID(stageElement, eventID).filter(isVisibleCalendarEventElement);
	for (const eventElement of eventElements) {
		const focusTarget = eventElement.matches(eventFocusTargetSelector)
			? eventElement
			: eventElement.querySelector<HTMLElement>(eventFocusTargetSelector);
		if (focusTarget && isVisibleCalendarEventElement(focusTarget)) return focusTarget;
	}
	return null;
}

function focusElement(element: HTMLElement | null): boolean {
	if (!element?.isConnected || !isVisibleCalendarEventElement(element)) return false;
	element.focus({ preventScroll: true });
	return document.activeElement === element;
}

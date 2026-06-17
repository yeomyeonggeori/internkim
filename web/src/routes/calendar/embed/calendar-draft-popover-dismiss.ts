import { isDraftPopoverValid } from './calendar-draft-popover-state';
import type { DraftPopoverState } from './calendar-draft-popover-state';

export type CalendarDraftPopoverDismissOptions = {
	stageElement: HTMLElement;
	getDraftPopover: () => DraftPopoverState | null;
	refreshDraftPopoverAnchor: () => boolean;
	saveDraftPopover: () => Promise<void>;
	cancelDraftPopover: () => Promise<void>;
	clearSelectedEvent: () => void;
};

export function installCalendarDraftPopoverDismiss(options: CalendarDraftPopoverDismissOptions): () => void {
	let shouldSuppressNextClick = false;
	let isDismissing = false;
	let refreshFrame: number | null = null;

	const dismissPopover = (popover: DraftPopoverState): void => {
		if (isDismissing) return;
		isDismissing = true;
		const dismissAction = isDraftPopoverValid(popover) ? options.saveDraftPopover : options.cancelDraftPopover;
		void dismissAction().finally(() => {
			isDismissing = false;
		});
	};

	const handlePointerDown = (event: PointerEvent): void => {
		const popover = options.getDraftPopover();
		if (!popover || !(event.target instanceof Element)) return;
		if (event.target.closest('.calendar-draft-popover')) return;
		const isInsideCalendarStage = options.stageElement.contains(event.target);
		const isCalendarEvent = Boolean(event.target.closest('.df-event, .df-month-segment-event'));
		dismissPopover(popover);
		if (!isCalendarEvent) options.clearSelectedEvent();
		if (!isInsideCalendarStage || isCalendarEvent) return;
		shouldSuppressNextClick = true;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
	};

	const handleClick = (event: MouseEvent): void => {
		if (!shouldSuppressNextClick) return;
		shouldSuppressNextClick = false;
		if (event.target instanceof Element && event.target.closest('.calendar-draft-popover')) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
	};

	const handleScroll = (event: Event): void => {
		const popover = options.getDraftPopover();
		if (!popover) return;
		if (event.target instanceof Element && event.target.closest('.calendar-draft-popover')) return;
		schedulePopoverAnchorRefresh();
	};

	const handleWheel = (event: WheelEvent): void => {
		const popover = options.getDraftPopover();
		if (!popover) return;
		schedulePopoverAnchorRefresh();
	};

	const schedulePopoverAnchorRefresh = (): void => {
		if (refreshFrame !== null) cancelAnimationFrame(refreshFrame);
		refreshFrame = requestAnimationFrame(() => {
			refreshFrame = requestAnimationFrame(() => {
				refreshFrame = null;
				const isAnchorVisible = options.refreshDraftPopoverAnchor();
				if (!isAnchorVisible) options.clearSelectedEvent();
			});
		});
	};

	document.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('click', handleClick, true);
	document.addEventListener('scroll', handleScroll, true);
	document.addEventListener('wheel', handleWheel, true);

	return () => {
		document.removeEventListener('pointerdown', handlePointerDown, true);
		document.removeEventListener('click', handleClick, true);
		document.removeEventListener('scroll', handleScroll, true);
		document.removeEventListener('wheel', handleWheel, true);
		if (refreshFrame !== null) cancelAnimationFrame(refreshFrame);
	};
}

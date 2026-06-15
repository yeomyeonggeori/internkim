// 캘린더 초안 팝오버의 외부 클릭 닫기와 재생성 방지를 처리합니다.
import { isDraftPopoverValid } from './calendar-draft-popover-state';
import type { DraftPopoverState } from './calendar-draft-popover-state';

export type CalendarDraftPopoverDismissOptions = {
	stageElement: HTMLElement;
	getDraftPopover: () => DraftPopoverState | null;
	saveDraftPopover: () => Promise<void>;
	cancelDraftPopover: () => Promise<void>;
};

export function installCalendarDraftPopoverDismiss(options: CalendarDraftPopoverDismissOptions): () => void {
	let shouldSuppressNextClick = false;
	let isDismissing = false;

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

	document.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('click', handleClick, true);

	return () => {
		document.removeEventListener('pointerdown', handlePointerDown, true);
		document.removeEventListener('click', handleClick, true);
	};
}

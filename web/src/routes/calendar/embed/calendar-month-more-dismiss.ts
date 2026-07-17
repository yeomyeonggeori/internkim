type MonthMoreDismissContext = {
	isActive: () => boolean;
	clearActiveMorePlacement: () => void;
	getActivePopoverElement: () => HTMLElement | null;
	getActiveTriggerElement: () => HTMLElement | null;
};

const nestedEditorSelector = '.calendar-draft-popover, .calendar-mobile-event-editor';

export function installMonthMoreDismiss(context: MonthMoreDismissContext): () => void {
	if (!context.isActive()) return () => {};
	const initialFocusFrame = requestAnimationFrame(() => {
		if (!context.isActive() || document.querySelector(nestedEditorSelector)) return;
		context
			.getActivePopoverElement()
			?.querySelector<HTMLElement>('.calendar-month-more-popover-event')
			?.focus({ preventScroll: true });
	});

	function handlePointerDown(pointerEvent: PointerEvent): void {
		if (!(pointerEvent.target instanceof Element)) return;
		if (
			pointerEvent.target.closest(
				`.calendar-month-more-popover, .calendar-month-more-button, ${nestedEditorSelector}`
			)
		) {
			return;
		}
		context.clearActiveMorePlacement();
	}

	function handleKeyDown(keyboardEvent: KeyboardEvent): void {
		if (keyboardEvent.key !== 'Escape') return;
		if (keyboardEvent.target instanceof Element && keyboardEvent.target.closest(nestedEditorSelector)) return;
		const triggerElement = context.getActiveTriggerElement();
		keyboardEvent.preventDefault();
		keyboardEvent.stopPropagation();
		keyboardEvent.stopImmediatePropagation();
		context.clearActiveMorePlacement();
		requestAnimationFrame(() => triggerElement?.isConnected && triggerElement.focus({ preventScroll: true }));
	}

	document.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('keydown', handleKeyDown, true);
	return () => {
		cancelAnimationFrame(initialFocusFrame);
		document.removeEventListener('pointerdown', handlePointerDown, true);
		document.removeEventListener('keydown', handleKeyDown, true);
	};
}

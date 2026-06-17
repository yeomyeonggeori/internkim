type MonthMoreDismissContext = {
	isActive: () => boolean;
	clearActiveMorePlacement: () => void;
};

export function installMonthMoreDismiss(context: MonthMoreDismissContext): () => void {
	if (!context.isActive()) return () => {};

	function handlePointerDown(pointerEvent: PointerEvent): void {
		if (!(pointerEvent.target instanceof Element)) return;
		if (pointerEvent.target.closest('.calendar-month-more-popover, .calendar-month-more-button')) return;
		context.clearActiveMorePlacement();
	}

	function handleKeyDown(keyboardEvent: KeyboardEvent): void {
		if (keyboardEvent.key === 'Escape') context.clearActiveMorePlacement();
	}

	document.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('keydown', handleKeyDown, true);
	return () => {
		document.removeEventListener('pointerdown', handlePointerDown, true);
		document.removeEventListener('keydown', handleKeyDown, true);
	};
}

import type { DraftPopoverState } from './calendar-draft-popover-state';

export type CalendarKeyboardSaveContext = {
	getDraftPopover: () => DraftPopoverState | null;
	saveDraftPopover: () => Promise<void>;
};

export function installCalendarKeyboardSave(context: CalendarKeyboardSaveContext): () => void {
	const handleKeydown = (event: KeyboardEvent): void => {
		if (event.key !== 'Enter' || event.isComposing || event.defaultPrevented) return;
		if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return;
		if (!context.getDraftPopover()) return;
		if (document.activeElement?.closest('[data-slot="popover-content"]')) return;
		event.preventDefault();
		void context.saveDraftPopover();
	};
	window.addEventListener('keydown', handleKeydown);
	return () => {
		window.removeEventListener('keydown', handleKeydown);
	};
}

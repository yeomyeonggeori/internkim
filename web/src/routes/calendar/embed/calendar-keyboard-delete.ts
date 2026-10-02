import { calendarOwnsKeyboardEvent } from './calendar-keyboard-scope';
import type { DraftPopoverState } from './calendar-draft-popover-state';

export type CalendarKeyboardDeleteContext = {
	getSelectedEventID: () => string | null;
	getDraftPopover: () => DraftPopoverState | null;
	deleteSelectedEvent: (eventID: string) => void;
};

export function installCalendarKeyboardDelete(context: CalendarKeyboardDeleteContext): () => void {
	const handleKeydown = (event: KeyboardEvent): void => {
		if (!calendarOwnsKeyboardEvent(event)) return;
		if (!isDeleteKey(event)) return;
		if (event.defaultPrevented || event.isComposing || event.metaKey || event.ctrlKey || event.altKey) return;
		if (isEditableKeyboardTarget(event.target, document.activeElement)) return;
		const popover = context.getDraftPopover();
		const selectedEventID = context.getSelectedEventID() ?? (popover?.mode === 'edit' ? popover.eventID : null);
		if (!selectedEventID) return;
		event.preventDefault();
		event.stopPropagation();
		context.deleteSelectedEvent(selectedEventID);
	};
	window.addEventListener('keydown', handleKeydown);
	return () => {
		window.removeEventListener('keydown', handleKeydown);
	};
}

function isDeleteKey(event: KeyboardEvent): boolean {
	return event.key === 'Backspace' || event.key === 'Delete';
}

function isEditableKeyboardTarget(target: EventTarget | null, activeElement: Element | null): boolean {
	return isEditableElement(target) || isEditableElement(activeElement);
}

function isEditableElement(value: EventTarget | Element | null): boolean {
	if (!(value instanceof HTMLElement)) return false;
	if (value.isContentEditable) return true;
	return value instanceof HTMLInputElement || value instanceof HTMLTextAreaElement || value instanceof HTMLSelectElement;
}

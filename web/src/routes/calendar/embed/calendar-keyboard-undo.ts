export type CalendarKeyboardUndoContext = {
	undoLastDelete: () => boolean;
};

export function installCalendarKeyboardUndo(context: CalendarKeyboardUndoContext): () => void {
	const handleKeydown = (event: KeyboardEvent): void => {
		if (!isUndoShortcut(event) || event.defaultPrevented || event.isComposing) return;
		if (isEditableKeyboardTarget(event.target, document.activeElement)) return;
		if (!context.undoLastDelete()) return;
		event.preventDefault();
		event.stopPropagation();
	};
	window.addEventListener('keydown', handleKeydown);
	return () => {
		window.removeEventListener('keydown', handleKeydown);
	};
}

function isUndoShortcut(event: KeyboardEvent): boolean {
	if (event.key.toLowerCase() !== 'z' || event.shiftKey) return false;
	return event.metaKey || event.ctrlKey;
}

function isEditableKeyboardTarget(target: EventTarget | null, activeElement: Element | null): boolean {
	return isEditableElement(target) || isEditableElement(activeElement);
}

function isEditableElement(value: EventTarget | Element | null): boolean {
	if (!(value instanceof HTMLElement)) return false;
	if (value.isContentEditable) return true;
	return value instanceof HTMLInputElement || value instanceof HTMLTextAreaElement || value instanceof HTMLSelectElement;
}

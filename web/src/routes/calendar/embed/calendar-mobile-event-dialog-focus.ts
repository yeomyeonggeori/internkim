import { shouldDeferCalendarDialogEscape } from './calendar-dialog-escape';

type CalendarMobileEventDialogFocusOptions = {
	dialogElement: HTMLElement;
	initialFocusElement: HTMLElement | undefined;
	closeDialog: () => void;
};

const focusableElementSelector = [
	'button:not([disabled])',
	'input:not([disabled])',
	'select:not([disabled])',
	'textarea:not([disabled])',
	'[href]',
	'[tabindex]:not([tabindex="-1"])'
].join(',');

export function installCalendarMobileEventDialogFocus(options: CalendarMobileEventDialogFocusOptions): () => void {
	const returnFocusElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
	const handleKeydown = (event: KeyboardEvent): void => {
		if (event.key === 'Escape') {
			if (shouldDeferCalendarDialogEscape(event.target)) return;
			event.preventDefault();
			event.stopPropagation();
			options.closeDialog();
			return;
		}
		if (event.key !== 'Tab') return;
		trapDialogFocus(options.dialogElement, event);
	};

	options.dialogElement.addEventListener('keydown', handleKeydown, true);
	(options.initialFocusElement ?? firstFocusableElement(options.dialogElement) ?? options.dialogElement).focus({
		preventScroll: true
	});

	return () => {
		options.dialogElement.removeEventListener('keydown', handleKeydown, true);
		if (returnFocusElement?.isConnected) returnFocusElement.focus({ preventScroll: true });
	};
}

function trapDialogFocus(dialogElement: HTMLElement, event: KeyboardEvent): void {
	const focusableElements = visibleFocusableElements(dialogElement);
	const firstElement = focusableElements[0];
	const lastElement = focusableElements.at(-1);
	if (!firstElement || !lastElement) {
		event.preventDefault();
		dialogElement.focus({ preventScroll: true });
		return;
	}
	const activeElement = document.activeElement;
	if (event.shiftKey && activeElement === firstElement) {
		event.preventDefault();
		lastElement.focus({ preventScroll: true });
		return;
	}
	if (!event.shiftKey && activeElement === lastElement) {
		event.preventDefault();
		firstElement.focus({ preventScroll: true });
	}
}

function firstFocusableElement(dialogElement: HTMLElement): HTMLElement | null {
	return visibleFocusableElements(dialogElement)[0] ?? null;
}

function visibleFocusableElements(dialogElement: HTMLElement): HTMLElement[] {
	return Array.from(dialogElement.querySelectorAll<HTMLElement>(focusableElementSelector)).filter(
		(element) => element.getClientRects().length > 0
	);
}

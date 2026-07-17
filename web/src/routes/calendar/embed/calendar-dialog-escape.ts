const expandedComboboxSelector = '[role="combobox"][aria-expanded="true"]';

export function shouldDeferCalendarDialogEscape(target: EventTarget | null): boolean {
	return target instanceof Element && Boolean(target.closest(expandedComboboxSelector));
}

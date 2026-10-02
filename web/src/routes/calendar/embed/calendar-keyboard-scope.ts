export function calendarOwnsKeyboardEvent(event: KeyboardEvent): boolean {
	if (event.defaultPrevented || event.isComposing) return false;
	const dialogs = document.querySelectorAll<HTMLElement>('[role="dialog"], [role="alertdialog"]');
	if (Array.from(dialogs).some((dialog) => !dialog.classList.contains('calendar-draft-popover') && dialog.getClientRects().length > 0)) return false;
	const target = event.target instanceof Element ? event.target : document.activeElement;
	if (!target || target === document.body || target === document.documentElement) return true;
	return !!target.closest('.calendar-page, .calendar-draft-popover');
}

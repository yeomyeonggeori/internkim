export function isPlainShortcut(event: KeyboardEvent, code: string): boolean {
	if (event.code !== code) return false;
	if (event.altKey || event.shiftKey || event.metaKey || event.ctrlKey || event.repeat) return false;
	return !isEditableTarget(event.target);
}

function isEditableTarget(target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement)) return false;
	if (target.isContentEditable) return true;
	return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

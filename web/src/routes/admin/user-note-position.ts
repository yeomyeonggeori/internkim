export type NoteEditorPosition = {
	left: number;
	top: number;
};

const editorWidth = 320;
const editorHeight = 220;
const viewportPadding = 12;

export function noteEditorPositionFromButton(target: EventTarget | null): NoteEditorPosition {
	if (!(target instanceof HTMLElement)) return { left: viewportPadding, top: viewportPadding };
	const rect = target.getBoundingClientRect();
	return noteEditorPositionFromRect(rect, window.innerWidth, window.innerHeight);
}

export function noteEditorPositionFromRect(rect: DOMRect | Pick<DOMRect, 'left' | 'top' | 'bottom'>, viewportWidth: number, viewportHeight: number): NoteEditorPosition {
	const left = Math.min(Math.max(rect.left, viewportPadding), viewportWidth - editorWidth - viewportPadding);
	const preferredTop = rect.top - editorHeight - 8;
	const fallbackTop = rect.bottom + 8;
	const top = preferredTop >= viewportPadding
		? preferredTop
		: Math.min(fallbackTop, viewportHeight - editorHeight - viewportPadding);
	return {
		left: Math.max(left, viewportPadding),
		top: Math.max(top, viewportPadding)
	};
}

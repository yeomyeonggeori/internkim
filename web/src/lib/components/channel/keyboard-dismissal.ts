const scrollBackToDismissPixels = 120;
const dragToDismissPixels = 10;
const typingElements = 'input, textarea, [contenteditable="true"]';

export function typingElementOutside(node: HTMLElement): HTMLElement | null {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement) || !active.matches(typingElements)) return null;
	return node.contains(active) ? null : active;
}

function dismissKeyboardOnceDragged(node: HTMLElement, isFarEnough: (draggedPixels: number) => boolean): () => void {
	let startClientY: number | undefined;

	const handleTouchStart = (event: TouchEvent): void => {
		startClientY = event.touches.length === 1 ? event.touches[0].clientY : undefined;
	};

	const handleTouchMove = (event: TouchEvent): void => {
		if (startClientY === undefined || event.touches.length !== 1) return;
		if (!isFarEnough(event.touches[0].clientY - startClientY)) return;
		startClientY = undefined;
		typingElementOutside(node)?.blur();
	};

	node.addEventListener('touchstart', handleTouchStart, { passive: true });
	node.addEventListener('touchmove', handleTouchMove, { passive: true });

	return () => {
		node.removeEventListener('touchstart', handleTouchStart);
		node.removeEventListener('touchmove', handleTouchMove);
	};
}

export function dismissKeyboardOnScrollBack(node: HTMLElement): { destroy(): void } {
	return { destroy: dismissKeyboardOnceDragged(node, (draggedPixels) => draggedPixels >= scrollBackToDismissPixels) };
}

export function dismissKeyboardOnDrag(node: HTMLElement): () => void {
	return dismissKeyboardOnceDragged(node, (draggedPixels) => Math.abs(draggedPixels) >= dragToDismissPixels);
}

const keyboardDismissalMilliseconds = 200;

export function openOnceKeyboardDismissed(open: () => void): void {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement) || !active.matches(typingElements)) {
		open();
		return;
	}
	active.blur();
	setTimeout(open, keyboardDismissalMilliseconds);
}

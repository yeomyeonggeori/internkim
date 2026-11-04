const scrollBackToDismissPixels = 40;
const typingElements = 'input, textarea, [contenteditable="true"]';

export function typingElementOutside(node: HTMLElement): HTMLElement | null {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement) || !active.matches(typingElements)) return null;
	return node.contains(active) ? null : active;
}

export function dismissKeyboardOnScrollBack(node: HTMLElement): { destroy(): void } {
	let startClientY: number | undefined;

	const handleTouchStart = (event: TouchEvent): void => {
		startClientY = event.touches.length === 1 ? event.touches[0].clientY : undefined;
	};

	const handleTouchMove = (event: TouchEvent): void => {
		if (startClientY === undefined || event.touches.length !== 1) return;
		if (event.touches[0].clientY - startClientY < scrollBackToDismissPixels) return;
		startClientY = undefined;
		typingElementOutside(node)?.blur();
	};

	node.addEventListener('touchstart', handleTouchStart, { passive: true });
	node.addEventListener('touchmove', handleTouchMove, { passive: true });

	return {
		destroy() {
			node.removeEventListener('touchstart', handleTouchStart);
			node.removeEventListener('touchmove', handleTouchMove);
		}
	};
}

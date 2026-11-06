function swallowNextClick(): void {
	const swallow = (event: MouseEvent): void => {
		event.preventDefault();
		event.stopImmediatePropagation();
		stop();
	};
	const stop = (): void => {
		window.removeEventListener('click', swallow, true);
		window.removeEventListener('pointerdown', stop, true);
	};
	window.addEventListener('click', swallow, true);
	window.addEventListener('pointerdown', stop, true);
}

export function swallowClickAfterOutsidePress(isInside: (target: Node) => boolean): () => void {
	const handlePointerDown = (event: PointerEvent): void => {
		if (event.pointerType === 'mouse' && event.target instanceof Node && !isInside(event.target)) swallowNextClick();
	};
	window.addEventListener('pointerdown', handlePointerDown, true);
	return () => window.removeEventListener('pointerdown', handlePointerDown, true);
}

export function closeOnOutsidePress(isInside: (target: Node) => boolean, close: () => void): () => void {
	const handlePointerDown = (event: PointerEvent): void => {
		if (!(event.target instanceof Node) || isInside(event.target)) return;
		swallowNextClick();
		close();
	};
	window.addEventListener('pointerdown', handlePointerDown, true);
	return () => window.removeEventListener('pointerdown', handlePointerDown, true);
}

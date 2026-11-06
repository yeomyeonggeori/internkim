export const touchHoldMilliseconds = 400;
export const touchHoldDriftPixels = 10;

export function openContextMenuOnTouchHold(node: HTMLElement): { destroy(): void } {
	let holdTimer: ReturnType<typeof setTimeout> | undefined;
	let startX = 0;
	let startY = 0;

	const cancelHold = (): void => {
		clearTimeout(holdTimer);
		holdTimer = undefined;
	};

	const openMenu = (): void => {
		holdTimer = undefined;
		node.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: startX, clientY: startY }));
	};

	const handlePointerDown = (event: PointerEvent): void => {
		cancelHold();
		if (event.pointerType === 'mouse') return;
		startX = event.clientX;
		startY = event.clientY;
		holdTimer = setTimeout(openMenu, touchHoldMilliseconds);
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (holdTimer === undefined) return;
		if (Math.hypot(event.clientX - startX, event.clientY - startY) > touchHoldDriftPixels) cancelHold();
	};

	node.addEventListener('pointerdown', handlePointerDown);
	node.addEventListener('pointermove', handlePointerMove);
	node.addEventListener('pointerup', cancelHold);
	node.addEventListener('pointercancel', cancelHold);

	return {
		destroy() {
			cancelHold();
			node.removeEventListener('pointerdown', handlePointerDown);
			node.removeEventListener('pointermove', handlePointerMove);
			node.removeEventListener('pointerup', cancelHold);
			node.removeEventListener('pointercancel', cancelHold);
		}
	};
}

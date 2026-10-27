export type HoldToReorderMoves = {
	onStart: () => void;
	onMove: (under: Element | null) => void;
	onEnd: () => void;
	onCancel: () => void;
};

export type HoldToReorderOptions = HoldToReorderMoves & {
	isMenuOpen: boolean;
	closeMenu: () => void;
};

type Point = { x: number; y: number };

const distanceBeforeReordering = 8;

export function holdToReorder(
	node: HTMLElement,
	initialOptions: HoldToReorderOptions
): { update(nextOptions: HoldToReorderOptions): void; destroy(): void } {
	let options = initialOptions;
	let touchID: number | null = null;
	let pressedAt: Point | null = null;
	let heldAt: Point | null = null;
	let isReordering = false;

	const trackedTouch = (event: TouchEvent): Touch | undefined =>
		Array.from(event.changedTouches).find((touch) => touch.identifier === touchID);

	const release = (isDropped: boolean): void => {
		if (isReordering && isDropped) options.onEnd();
		if (isReordering && !isDropped) options.onCancel();
		touchID = null;
		pressedAt = null;
		heldAt = null;
		isReordering = false;
	};

	const handleTouchStart = (event: TouchEvent): void => {
		const touch = event.changedTouches[0];
		if (event.touches.length !== 1 || !touch) {
			release(false);
			return;
		}
		touchID = touch.identifier;
		pressedAt = { x: touch.clientX, y: touch.clientY };
		heldAt = null;
		isReordering = false;
	};

	const handleTouchMove = (event: TouchEvent): void => {
		const touch = trackedTouch(event);
		if (!touch || !heldAt) return;
		event.preventDefault();
		if (!isReordering) {
			if (Math.hypot(touch.clientX - heldAt.x, touch.clientY - heldAt.y) < distanceBeforeReordering) return;
			isReordering = true;
			options.closeMenu();
			options.onStart();
		}
		options.onMove(document.elementFromPoint(touch.clientX, touch.clientY));
	};

	const handleTouchEnd = (event: TouchEvent): void => {
		if (!trackedTouch(event)) return;
		release(event.type === 'touchend');
	};

	node.addEventListener('touchstart', handleTouchStart, { passive: true });
	node.addEventListener('touchmove', handleTouchMove, { passive: false });
	node.addEventListener('touchend', handleTouchEnd);
	node.addEventListener('touchcancel', handleTouchEnd);

	return {
		update(nextOptions) {
			const hasJustOpened = nextOptions.isMenuOpen && !options.isMenuOpen;
			options = nextOptions;
			if (hasJustOpened && pressedAt) heldAt = pressedAt;
		},
		destroy() {
			node.removeEventListener('touchstart', handleTouchStart);
			node.removeEventListener('touchmove', handleTouchMove);
			node.removeEventListener('touchend', handleTouchEnd);
			node.removeEventListener('touchcancel', handleTouchEnd);
		}
	};
}

import { feelHaptic } from '$lib/native-shell/haptics';

export type ReactionChipHoldOptions = {
	onHold: () => void;
};

const holdMilliseconds = 700;
const allowedDriftPixels = 10;

export function reactionChipHold(
	node: HTMLElement,
	initialOptions: ReactionChipHoldOptions
): { update(nextOptions: ReactionChipHoldOptions): void; destroy(): void } {
	let options = initialOptions;
	let holdTimer: ReturnType<typeof setTimeout> | undefined;
	let startX = 0;
	let startY = 0;
	let shouldSwallowNextClick = false;

	const cancelHold = (): void => {
		clearTimeout(holdTimer);
		holdTimer = undefined;
	};

	const hold = (): void => {
		cancelHold();
		shouldSwallowNextClick = true;
		feelHaptic('touch');
		options.onHold();
	};

	const handlePointerDown = (event: PointerEvent): void => {
		shouldSwallowNextClick = false;
		if (event.pointerType === 'mouse') return;
		event.stopPropagation();
		startX = event.clientX;
		startY = event.clientY;
		cancelHold();
		holdTimer = setTimeout(hold, holdMilliseconds);
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (holdTimer === undefined) return;
		if (Math.hypot(event.clientX - startX, event.clientY - startY) > allowedDriftPixels) cancelHold();
	};

	const handleContextMenu = (event: MouseEvent): void => {
		event.preventDefault();
		event.stopPropagation();
		if (shouldSwallowNextClick) return;
		cancelHold();
		options.onHold();
	};

	const handleClick = (event: MouseEvent): void => {
		if (!shouldSwallowNextClick) return;
		shouldSwallowNextClick = false;
		event.preventDefault();
		event.stopImmediatePropagation();
	};

	node.addEventListener('pointerdown', handlePointerDown);
	node.addEventListener('pointermove', handlePointerMove);
	node.addEventListener('pointerup', cancelHold);
	node.addEventListener('pointercancel', cancelHold);
	node.addEventListener('contextmenu', handleContextMenu);
	node.addEventListener('click', handleClick, true);

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			cancelHold();
			node.removeEventListener('pointerdown', handlePointerDown);
			node.removeEventListener('pointermove', handlePointerMove);
			node.removeEventListener('pointerup', cancelHold);
			node.removeEventListener('pointercancel', cancelHold);
			node.removeEventListener('contextmenu', handleContextMenu);
			node.removeEventListener('click', handleClick, true);
		}
	};
}

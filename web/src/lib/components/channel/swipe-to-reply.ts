export const swipeReplyThresholdPixels = 56;
export const swipeReplyLimitPixels = 80;
export const swipeDecisionPixels = 8;

export type SwipeState = {
	offsetPixels: number;
	isArmed: boolean;
	isHorizontal: boolean;
};

export type SwipeToReplyOptions = {
	onReply: () => void;
	disabled?: boolean;
};

function clamp(value: number, minimum: number, maximum: number): number {
	return Math.min(Math.max(value, minimum), maximum);
}

export function swipeStateOf(deltaX: number, deltaY: number): SwipeState {
	const isHorizontal = Math.abs(deltaX) > Math.abs(deltaY);
	const offsetPixels = isHorizontal && deltaX < 0 ? clamp(-deltaX, 0, swipeReplyLimitPixels) : 0;
	return {
		offsetPixels,
		isArmed: offsetPixels >= swipeReplyThresholdPixels,
		isHorizontal
	};
}

export function swipeToReply(
	node: HTMLElement,
	initialOptions: SwipeToReplyOptions
): { update(nextOptions: SwipeToReplyOptions): void; destroy(): void } {
	let options = initialOptions;
	let activePointerID: number | undefined;
	let startClientX = 0;
	let startClientY = 0;
	let isAbandoned = false;
	let isSwiping = false;
	let isArmed = false;
	const previousTouchAction = node.style.touchAction;
	node.style.touchAction = 'pan-y';

	const clearSwipeStyle = (): void => {
		node.style.removeProperty('--swipe-offset');
		node.style.removeProperty('--swipe-progress');
		node.removeAttribute('data-swipe-armed');
	};

	const handlePointerDown = (event: PointerEvent): void => {
		if (options.disabled || event.pointerType !== 'touch') return;
		activePointerID = event.pointerId;
		startClientX = event.clientX;
		startClientY = event.clientY;
		isAbandoned = false;
		isSwiping = false;
		isArmed = false;
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (options.disabled || event.pointerType !== 'touch') return;
		if (activePointerID !== event.pointerId || isAbandoned) return;
		const deltaX = event.clientX - startClientX;
		const deltaY = event.clientY - startClientY;
		const hasTravelledEnoughToReadItsDirection = isSwiping || Math.hypot(deltaX, deltaY) >= swipeDecisionPixels;
		if (!hasTravelledEnoughToReadItsDirection) return;
		const state = swipeStateOf(deltaX, isSwiping ? 0 : deltaY);
		if (!isSwiping && !state.isHorizontal) {
			isAbandoned = true;
			return;
		}
		isSwiping = true;
		isArmed = state.isArmed;
		node.style.setProperty('--swipe-offset', `${-state.offsetPixels}px`);
		node.style.setProperty('--swipe-progress', `${Math.min(state.offsetPixels / swipeReplyThresholdPixels, 1)}`);
		node.setAttribute('data-swipe-armed', state.isArmed ? 'true' : 'false');
	};

	const handlePointerUp = (event: PointerEvent): void => {
		if (options.disabled || event.pointerType !== 'touch') return;
		if (activePointerID !== event.pointerId) return;
		const shouldReply = isArmed;
		activePointerID = undefined;
		isAbandoned = false;
		isSwiping = false;
		isArmed = false;
		clearSwipeStyle();
		if (shouldReply) options.onReply();
	};

	const handlePointerCancel = (event: PointerEvent): void => {
		if (options.disabled) return;
		if (activePointerID !== event.pointerId) return;
		activePointerID = undefined;
		isAbandoned = false;
		isSwiping = false;
		isArmed = false;
		clearSwipeStyle();
	};

	node.addEventListener('pointerdown', handlePointerDown);
	node.addEventListener('pointermove', handlePointerMove);
	node.addEventListener('pointerup', handlePointerUp);
	node.addEventListener('pointercancel', handlePointerCancel);

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			node.removeEventListener('pointerdown', handlePointerDown);
			node.removeEventListener('pointermove', handlePointerMove);
			node.removeEventListener('pointerup', handlePointerUp);
			node.removeEventListener('pointercancel', handlePointerCancel);
			clearSwipeStyle();
			node.style.touchAction = previousTouchAction;
		}
	};
}

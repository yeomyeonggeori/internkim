import { swipeDecisionPixels, swipeReplyThresholdPixels } from '$lib/components/channel/swipe-to-reply';

export type SwipeToOpenChannelListOptions = {
	onOpen: () => void;
	disabled: boolean;
};

function startsInsideSidewaysScroller(target: EventTarget | null, boundary: HTMLElement): boolean {
	let element = target instanceof Element ? target : null;
	while (element && element !== boundary) {
		const overflowX = getComputedStyle(element).overflowX;
		const scrollsSideways = overflowX === 'auto' || overflowX === 'scroll';
		if (scrollsSideways && element.scrollWidth > element.clientWidth) return true;
		element = element.parentElement;
	}
	return false;
}

export function swipeToOpenChannelList(
	node: HTMLElement,
	initialOptions: SwipeToOpenChannelListOptions
): { update(nextOptions: SwipeToOpenChannelListOptions): void; destroy(): void } {
	let options = initialOptions;
	let activePointerID: number | undefined;
	let startClientX = 0;
	let startClientY = 0;
	let isAbandoned = false;
	let isSwiping = false;
	const previousTouchAction = node.style.touchAction;
	node.style.touchAction = 'pan-y';

	const forgetSwipe = (): void => {
		activePointerID = undefined;
		isAbandoned = false;
		isSwiping = false;
	};

	const handlePointerDown = (event: PointerEvent): void => {
		if (options.disabled || event.pointerType !== 'touch') return;
		if (startsInsideSidewaysScroller(event.target, node)) return;
		activePointerID = event.pointerId;
		startClientX = event.clientX;
		startClientY = event.clientY;
		isAbandoned = false;
		isSwiping = false;
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (activePointerID !== event.pointerId || isAbandoned || isSwiping) return;
		const deltaX = event.clientX - startClientX;
		const deltaY = event.clientY - startClientY;
		if (Math.hypot(deltaX, deltaY) < swipeDecisionPixels) return;
		if (deltaX > Math.abs(deltaY)) isSwiping = true;
		else isAbandoned = true;
	};

	const handlePointerUp = (event: PointerEvent): void => {
		if (activePointerID !== event.pointerId) return;
		const shouldOpen = isSwiping && !options.disabled && event.clientX - startClientX >= swipeReplyThresholdPixels;
		forgetSwipe();
		if (shouldOpen) options.onOpen();
	};

	const handlePointerCancel = (event: PointerEvent): void => {
		if (activePointerID !== event.pointerId) return;
		forgetSwipe();
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
			node.style.touchAction = previousTouchAction;
		}
	};
}

import { feelHaptic } from '$lib/native-shell/haptics';

export type TouchHoldOptions = {
	isMenuOpen: boolean;
};

export function swallowClickAfterTouchHold(
	node: HTMLElement,
	initialOptions: TouchHoldOptions
): { update(nextOptions: TouchHoldOptions): void; destroy(): void } {
	let isMenuOpen = initialOptions.isMenuOpen;
	let isTouchDown = false;
	let shouldSwallowNextClick = false;

	const handlePointerDown = (event: PointerEvent): void => {
		shouldSwallowNextClick = false;
		isTouchDown = event.pointerType !== 'mouse';
	};

	const handlePointerEnd = (): void => {
		isTouchDown = false;
	};

	const handleClick = (event: MouseEvent): void => {
		if (!shouldSwallowNextClick) return;
		shouldSwallowNextClick = false;
		event.preventDefault();
		event.stopPropagation();
	};

	node.addEventListener('pointerdown', handlePointerDown, true);
	node.addEventListener('pointerup', handlePointerEnd, true);
	node.addEventListener('pointercancel', handlePointerEnd, true);
	node.addEventListener('click', handleClick, true);

	return {
		update(nextOptions) {
			const hasJustOpened = nextOptions.isMenuOpen && !isMenuOpen;
			const hasJustClosed = !nextOptions.isMenuOpen && isMenuOpen;
			if (hasJustOpened && isTouchDown) {
				shouldSwallowNextClick = true;
				feelHaptic('touch');
			}
			if (hasJustClosed) shouldSwallowNextClick = false;
			isMenuOpen = nextOptions.isMenuOpen;
		},
		destroy() {
			node.removeEventListener('pointerdown', handlePointerDown, true);
			node.removeEventListener('pointerup', handlePointerEnd, true);
			node.removeEventListener('pointercancel', handlePointerEnd, true);
			node.removeEventListener('click', handleClick, true);
		}
	};
}

import { swipeDecisionPixels } from './swipe-to-reply';

export type OpenThreadOnTapOptions = {
	onOpen: () => void;
	disabled: boolean;
};

const elementsWithTheirOwnAction =
	'a, button, input, textarea, select, label, summary, [role="button"], [role="link"], [contenteditable="true"]';

function hasItsOwnAction(target: EventTarget | null, row: HTMLElement): boolean {
	if (!(target instanceof Element)) return false;
	const actionable = target.closest(elementsWithTheirOwnAction);
	return actionable !== null && row.contains(actionable);
}

function hasSelectedTextInside(row: HTMLElement): boolean {
	const selection = window.getSelection();
	if (selection === null || selection.isCollapsed) return false;
	return selection.rangeCount > 0 && selection.getRangeAt(0).intersectsNode(row);
}

export function openThreadOnTap(
	node: HTMLElement,
	initialOptions: OpenThreadOnTapOptions
): { update(nextOptions: OpenThreadOnTapOptions): void; destroy(): void } {
	let options = initialOptions;
	let startClientX = 0;
	let startClientY = 0;
	let hasTravelled = false;

	const handlePointerDown = (event: PointerEvent): void => {
		startClientX = event.clientX;
		startClientY = event.clientY;
		hasTravelled = false;
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (hasTravelled || event.buttons === 0) return;
		hasTravelled = Math.hypot(event.clientX - startClientX, event.clientY - startClientY) >= swipeDecisionPixels;
	};

	const handleClick = (event: MouseEvent): void => {
		if (options.disabled || event.defaultPrevented || hasTravelled) return;
		if (hasItsOwnAction(event.target, node) || hasSelectedTextInside(node)) return;
		options.onOpen();
	};

	node.addEventListener('pointerdown', handlePointerDown);
	node.addEventListener('pointermove', handlePointerMove);
	node.addEventListener('click', handleClick);

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			node.removeEventListener('pointerdown', handlePointerDown);
			node.removeEventListener('pointermove', handlePointerMove);
			node.removeEventListener('click', handleClick);
		}
	};
}

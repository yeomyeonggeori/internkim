import type { Action } from 'svelte/action';

const minimumBoardHeight = 320;
const bottomSpacing = 24;
const scrollableOverflowValues = new Set(['auto', 'scroll', 'overlay']);

export const flowTaskBoardViewportHeight: Action<HTMLElement> = (node) => {
	let animationFrameID = 0;
	const scrollTarget = findVerticalScrollTarget(node);
	const updateHeight = () => {
		if (animationFrameID) cancelAnimationFrame(animationFrameID);
		animationFrameID = requestAnimationFrame(() => {
			const viewport = visibleViewportBounds(scrollTarget);
			const unscrolledTop = viewport.top + boardOffsetWithinScrollTarget(node, scrollTarget);
			const height = Math.max(minimumBoardHeight, viewport.bottom - unscrolledTop - bottomSpacing);
			node.style.setProperty('--flow-task-board-height', `${Math.floor(height)}px`);
			animationFrameID = 0;
		});
	};
	const resizeObserver = new ResizeObserver(updateHeight);
	resizeObserver.observe(document.body);
	resizeObserver.observe(node);
	if (scrollTarget instanceof HTMLElement) resizeObserver.observe(scrollTarget);
	scrollTarget.addEventListener('scroll', updateHeight, { passive: true });
	window.addEventListener('resize', updateHeight);
	window.visualViewport?.addEventListener('resize', updateHeight);
	updateHeight();

	return {
		destroy() {
			if (animationFrameID) cancelAnimationFrame(animationFrameID);
			resizeObserver.disconnect();
			scrollTarget.removeEventListener('scroll', updateHeight);
			window.removeEventListener('resize', updateHeight);
			window.visualViewport?.removeEventListener('resize', updateHeight);
			node.style.removeProperty('--flow-task-board-height');
		}
	};
};

function boardOffsetWithinScrollTarget(node: HTMLElement, scrollTarget: Window | HTMLElement): number {
	const nodeTop = node.getBoundingClientRect().top;
	if (scrollTarget instanceof HTMLElement) {
		return nodeTop - scrollTarget.getBoundingClientRect().top + scrollTarget.scrollTop;
	}
	return nodeTop + window.scrollY;
}

function findVerticalScrollTarget(node: HTMLElement): Window | HTMLElement {
	let element = node.parentElement;
	while (element) {
		const overflowY = getComputedStyle(element).overflowY;
		if (scrollableOverflowValues.has(overflowY)) return element;
		element = element.parentElement;
	}
	return window;
}

function visibleViewportBounds(scrollTarget: Window | HTMLElement): { top: number; bottom: number } {
	if (scrollTarget instanceof HTMLElement) {
		const bounds = scrollTarget.getBoundingClientRect();
		return { top: bounds.top, bottom: bounds.bottom };
	}
	const visualViewport = window.visualViewport;
	if (visualViewport) {
		return {
			top: visualViewport.offsetTop,
			bottom: visualViewport.offsetTop + visualViewport.height
		};
	}
	return { top: 0, bottom: window.innerHeight };
}

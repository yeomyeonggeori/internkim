import type { Action } from 'svelte/action';

const minimumBoardHeight = 320;
const bottomSpacing = 24;

export const flowTaskBoardViewportHeight: Action<HTMLElement> = (node) => {
	let animationFrameID = 0;
	const updateHeight = () => {
		if (animationFrameID) cancelAnimationFrame(animationFrameID);
		animationFrameID = requestAnimationFrame(() => {
			const viewportHeight = window.visualViewport?.height ?? window.innerHeight;
			const top = node.getBoundingClientRect().top;
			const height = Math.max(minimumBoardHeight, viewportHeight - top - bottomSpacing);
			node.style.setProperty('--flow-task-board-height', `${Math.floor(height)}px`);
			animationFrameID = 0;
		});
	};
	const resizeObserver = new ResizeObserver(updateHeight);
	resizeObserver.observe(document.body);
	resizeObserver.observe(node);
	window.addEventListener('resize', updateHeight);
	window.visualViewport?.addEventListener('resize', updateHeight);
	updateHeight();

	return {
		destroy() {
			if (animationFrameID) cancelAnimationFrame(animationFrameID);
			resizeObserver.disconnect();
			window.removeEventListener('resize', updateHeight);
			window.visualViewport?.removeEventListener('resize', updateHeight);
			node.style.removeProperty('--flow-task-board-height');
		}
	};
};

import type { Action } from 'svelte/action';

type ScrollOverflowChange = (hasScrollOverflow: boolean) => void;

const overflowTolerancePixels = 1;

export const observeScrollOverflow: Action<HTMLElement, ScrollOverflowChange> = (element, setScrollOverflow) => {
	let animationFrameID: number | undefined;

	const updateScrollOverflow = () => {
		if (animationFrameID !== undefined) {
			cancelAnimationFrame(animationFrameID);
		}

		animationFrameID = requestAnimationFrame(() => {
			animationFrameID = undefined;
			setScrollOverflow(element.scrollHeight > element.clientHeight + overflowTolerancePixels);
		});
	};

	const resizeObserver = new ResizeObserver(updateScrollOverflow);
	const mutationObserver = new MutationObserver(updateScrollOverflow);

	resizeObserver.observe(element);
	mutationObserver.observe(element, { childList: true, subtree: true, characterData: true });
	window.addEventListener('resize', updateScrollOverflow);
	updateScrollOverflow();

	return {
		destroy() {
			if (animationFrameID !== undefined) {
				cancelAnimationFrame(animationFrameID);
			}

			resizeObserver.disconnect();
			mutationObserver.disconnect();
			window.removeEventListener('resize', updateScrollOverflow);
		}
	};
};

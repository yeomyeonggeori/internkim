import type { Action } from 'svelte/action';

export type LightboxPinchZoomOptions = {
	isEnabled: () => boolean;
	zoomPercent: () => number;
	zoomTo: (percent: number) => void;
};

const wheelZoomSensitivity = 0.01;

function distanceBetweenFingers(touches: TouchList): number | null {
	const first = touches[0];
	const second = touches[1];
	if (touches.length !== 2 || !first || !second) return null;
	return Math.hypot(first.clientX - second.clientX, first.clientY - second.clientY);
}

function gestureScale(event: Event): number | null {
	return 'scale' in event && typeof event.scale === 'number' ? event.scale : null;
}

export const pinchesToZoom: Action<HTMLElement, LightboxPinchZoomOptions> = (node, options) => {
	let pinchStart: { distance: number; zoomPercent: number } | null = null;
	let gestureStartZoomPercent: number | null = null;

	function wheel(event: WheelEvent): void {
		if (!event.ctrlKey || !options.isEnabled()) return;
		event.preventDefault();
		options.zoomTo(options.zoomPercent() * Math.exp(-event.deltaY * wheelZoomSensitivity));
	}

	function touchStart(event: TouchEvent): void {
		const distance = distanceBetweenFingers(event.touches);
		pinchStart = distance && options.isEnabled() ? { distance, zoomPercent: options.zoomPercent() } : null;
	}

	function touchMove(event: TouchEvent): void {
		const distance = distanceBetweenFingers(event.touches);
		if (!pinchStart || !distance) return;
		event.preventDefault();
		options.zoomTo((pinchStart.zoomPercent * distance) / pinchStart.distance);
	}

	function touchEnd(event: TouchEvent): void {
		if (event.touches.length < 2) pinchStart = null;
	}

	function gestureStart(event: Event): void {
		if (!options.isEnabled()) return;
		event.preventDefault();
		gestureStartZoomPercent = options.zoomPercent();
	}

	function gestureChange(event: Event): void {
		const scale = gestureScale(event);
		if (gestureStartZoomPercent === null || scale === null) return;
		event.preventDefault();
		if (pinchStart) return;
		options.zoomTo(gestureStartZoomPercent * scale);
	}

	function gestureEnd(): void {
		gestureStartZoomPercent = null;
	}

	node.addEventListener('wheel', wheel, { passive: false });
	node.addEventListener('touchstart', touchStart, { passive: true });
	node.addEventListener('touchmove', touchMove, { passive: false });
	node.addEventListener('touchend', touchEnd);
	node.addEventListener('touchcancel', touchEnd);
	node.addEventListener('gesturestart', gestureStart);
	node.addEventListener('gesturechange', gestureChange);
	node.addEventListener('gestureend', gestureEnd);
	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			node.removeEventListener('wheel', wheel);
			node.removeEventListener('touchstart', touchStart);
			node.removeEventListener('touchmove', touchMove);
			node.removeEventListener('touchend', touchEnd);
			node.removeEventListener('touchcancel', touchEnd);
			node.removeEventListener('gesturestart', gestureStart);
			node.removeEventListener('gesturechange', gestureChange);
			node.removeEventListener('gestureend', gestureEnd);
		}
	};
};

import type { Action } from 'svelte/action';

export type TaskRoadmapSwipeDirection = -1 | 1;

export type TaskRoadmapSwipeOptions = {
	onSwipe: (direction: TaskRoadmapSwipeDirection) => void;
};

const wheelThresholdPixels = 80;
const wheelQuietMilliseconds = 180;
const touchThresholdPixels = 50;
const touchHorizontalDominance = 1.5;

type TouchOrigin = { x: number; y: number };

export const taskRoadmapSwipe: Action<HTMLElement, TaskRoadmapSwipeOptions> = (element, initialOptions) => {
	let options = initialOptions;
	let wheelDistance = 0;
	let wheelLocked = false;
	let wheelQuietTimer: ReturnType<typeof setTimeout> | undefined;
	let touchOrigin: TouchOrigin | undefined;

	function unlockWheelWhenQuiet(): void {
		clearTimeout(wheelQuietTimer);
		wheelQuietTimer = setTimeout(() => {
			wheelLocked = false;
			wheelDistance = 0;
		}, wheelQuietMilliseconds);
	}

	function wheel(event: WheelEvent): void {
		if (Math.abs(event.deltaX) <= Math.abs(event.deltaY)) return;
		event.preventDefault();
		unlockWheelWhenQuiet();
		if (wheelLocked) return;
		wheelDistance += event.deltaX;
		if (Math.abs(wheelDistance) < wheelThresholdPixels) return;
		options.onSwipe(wheelDistance > 0 ? 1 : -1);
		wheelDistance = 0;
		wheelLocked = true;
	}

	function touchStart(event: TouchEvent): void {
		const touch = event.touches[0];
		touchOrigin = event.touches.length === 1 && touch ? { x: touch.clientX, y: touch.clientY } : undefined;
	}

	function touchEnd(event: TouchEvent): void {
		const origin = touchOrigin;
		const touch = event.changedTouches[0];
		touchOrigin = undefined;
		if (!origin || !touch) return;
		const moveX = touch.clientX - origin.x;
		const moveY = touch.clientY - origin.y;
		if (Math.abs(moveX) < touchThresholdPixels || Math.abs(moveX) < Math.abs(moveY) * touchHorizontalDominance) return;
		options.onSwipe(moveX < 0 ? 1 : -1);
	}

	function touchCancel(): void {
		touchOrigin = undefined;
	}

	element.addEventListener('wheel', wheel, { passive: false });
	element.addEventListener('touchstart', touchStart, { passive: true });
	element.addEventListener('touchend', touchEnd, { passive: true });
	element.addEventListener('touchcancel', touchCancel, { passive: true });

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			clearTimeout(wheelQuietTimer);
			element.removeEventListener('wheel', wheel);
			element.removeEventListener('touchstart', touchStart);
			element.removeEventListener('touchend', touchEnd);
			element.removeEventListener('touchcancel', touchCancel);
		}
	};
};

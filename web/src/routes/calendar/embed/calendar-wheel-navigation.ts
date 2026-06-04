import { ViewType } from '@dayflow/svelte';

export type CalendarWheelNavigationOptions = {
	stageElement: HTMLElement;
	currentView: () => string;
	goToPrevious: () => void;
	goToNext: () => void;
};

const trackpadGestureGapMs = 80;
const wheelMinCooldownMs = 60;

export function installCalendarWheelNavigation(options: CalendarWheelNavigationOptions): () => void {
	let lastWheelAt = 0;
	let gestureLocked = false;

	const onWheel = (event: WheelEvent) => {
		if (options.currentView() !== ViewType.MONTH) return;

		event.preventDefault();

		const now = Date.now();
		const gap = now - lastWheelAt;
		lastWheelAt = now;

		const isDiscreteWheel = event.deltaMode !== 0;
		if (isDiscreteWheel) {
			if (gap < wheelMinCooldownMs) return;
			if (Math.abs(event.deltaY) < 1) return;
			if (event.deltaY > 0) options.goToNext();
			else options.goToPrevious();
			return;
		}

		if (gap > trackpadGestureGapMs) gestureLocked = false;
		if (gestureLocked) return;
		if (Math.abs(event.deltaY) < 1) return;

		gestureLocked = true;
		if (event.deltaY > 0) options.goToNext();
		else options.goToPrevious();
	};

	options.stageElement.addEventListener('wheel', onWheel, { passive: false });
	return () => options.stageElement.removeEventListener('wheel', onWheel);
}

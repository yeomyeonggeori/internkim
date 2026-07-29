type CalendarScrollSnapOptions = {
	getScrollElement: () => HTMLElement | null;
	getSnapOffsets: () => number[];
	damping?: number;
	animationMilliseconds?: number;
};

export type CalendarScrollSnap = {
	handleWheel: (wheelEvent: WheelEvent) => void;
	handleGestureEnd: () => void;
	handleScrollEnd: () => void;
	destroy: () => void;
};

const defaultDamping = 0.3;
const defaultAnimationMilliseconds = 220;
const momentumDecaySteps = 3;
const gestureGapMilliseconds = 140;

export function createCalendarScrollSnap(options: CalendarScrollSnapOptions): CalendarScrollSnap {
	const damping = options.damping ?? defaultDamping;
	const animationMilliseconds = options.animationMilliseconds ?? defaultAnimationMilliseconds;

	let animationFrame: number | null = null;
	let previousMagnitude = 0;
	let decayingSteps = 0;
	let previousWheelTime = 0;
	let isIgnoringMomentum = false;

	function wheelDeltaPixels(wheelEvent: WheelEvent, scrollElement: HTMLElement): number {
		if (wheelEvent.deltaMode === 1) return wheelEvent.deltaY * 16;
		if (wheelEvent.deltaMode === 2) return wheelEvent.deltaY * scrollElement.clientHeight;
		return wheelEvent.deltaY;
	}

	function handleWheel(wheelEvent: WheelEvent): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement || wheelEvent.ctrlKey) return;
		wheelEvent.preventDefault();

		const magnitude = Math.abs(wheelEvent.deltaY);
		const now = performance.now();
		const isNewGesture = now - previousWheelTime > gestureGapMilliseconds || magnitude > previousMagnitude + 0.5;
		previousWheelTime = now;

		if (isNewGesture) {
			isIgnoringMomentum = false;
			decayingSteps = 0;
			cancelAnimation();
		}
		if (isIgnoringMomentum) {
			previousMagnitude = magnitude;
			return;
		}

		scrollElement.scrollTop += wheelDeltaPixels(wheelEvent, scrollElement) * damping;
		decayingSteps = magnitude < previousMagnitude - 0.5 ? decayingSteps + 1 : 0;
		previousMagnitude = magnitude;
		if (decayingSteps < momentumDecaySteps) return;
		isIgnoringMomentum = true;
		snapToNearestOffset();
	}

	function handleGestureEnd(): void {
		isIgnoringMomentum = true;
		snapToNearestOffset();
	}

	function handleScrollEnd(): void {
		if (animationFrame !== null) return;
		snapToNearestOffset();
	}

	function snapToNearestOffset(): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement) return;
		const offsets = options.getSnapOffsets();
		if (offsets.length === 0) return;
		const nearestOffset = offsets.reduce(
			(closest, offset) => (Math.abs(offset - scrollElement.scrollTop) < Math.abs(closest - scrollElement.scrollTop) ? offset : closest),
			offsets[0]
		);
		if (Math.abs(nearestOffset - scrollElement.scrollTop) < 1) return;
		animateScrollTo(nearestOffset);
	}

	function animateScrollTo(targetScrollTop: number): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement) return;
		cancelAnimation();
		const startScrollTop = scrollElement.scrollTop;
		const scrollDistance = targetScrollTop - startScrollTop;
		const startTime = performance.now();
		const step = (now: number) => {
			const currentScrollElement = options.getScrollElement();
			if (!currentScrollElement) return;
			const progress = Math.min(1, (now - startTime) / animationMilliseconds);
			currentScrollElement.scrollTop = startScrollTop + scrollDistance * (1 - (1 - progress) ** 3);
			animationFrame = progress < 1 ? requestAnimationFrame(step) : null;
		};
		animationFrame = requestAnimationFrame(step);
	}

	function cancelAnimation(): void {
		if (animationFrame === null) return;
		cancelAnimationFrame(animationFrame);
		animationFrame = null;
	}

	return {
		handleWheel,
		handleGestureEnd,
		handleScrollEnd,
		destroy: cancelAnimation
	};
}

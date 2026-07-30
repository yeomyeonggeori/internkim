type CalendarScrollSnapOptions = {
	getScrollElement: () => HTMLElement | null;
	getSnapOffsets: () => number[];
	axis?: 'vertical' | 'horizontal';
	animationMilliseconds?: number;
	onSnapSettled?: (offset: number) => void;
	resolveSnapOffset?: (context: {
		currentOffset: number;
		gestureStartOffset: number;
		offsets: number[];
		gestureVelocity: number;
	}) => number;
};

export type CalendarScrollSnap = {
	handleWheel: (wheelEvent: WheelEvent) => void;
	handleGestureEnd: () => void;
	handleScrollEnd: () => void;
	handleTouchStart: () => void;
	destroy: () => void;
};

const defaultAnimationMilliseconds = 220;
const momentumDecaySteps = 3;
const gestureGapMilliseconds = 140;
const wheelFollowFactor = 0.28;
const idleSnapMilliseconds = 140;
const millisecondsPerSnapPixel = 0.45;
const maximumSnapMilliseconds = 420;

export function createCalendarScrollSnap(options: CalendarScrollSnapOptions): CalendarScrollSnap {
	const axis = options.axis ?? 'vertical';
	const animationMilliseconds = options.animationMilliseconds ?? defaultAnimationMilliseconds;

	let animationFrame: number | null = null;
	let settledOffset: number | null = null;
	let isIdleSnapped = false;
	let previousMagnitude = 0;
	let decayingSteps = 0;
	let previousWheelTime = 0;
	let isIgnoringMomentum = false;
	let gestureStartOffset = 0;
	let gestureStartTime = 0;
	let gestureLiftTime = 0;
	let gestureLiftOffset = 0;
	let wheelTargetOffset = 0;
	let wheelFrame: number | null = null;
	let idleSnapTimer: number | null = null;

	function wheelDelta(wheelEvent: WheelEvent): number {
		return axis === 'horizontal' ? wheelEvent.deltaX : wheelEvent.deltaY;
	}

	function wheelDeltaPixels(wheelEvent: WheelEvent, scrollElement: HTMLElement): number {
		const delta = wheelDelta(wheelEvent);
		if (wheelEvent.deltaMode === 1) return delta * 16;
		if (wheelEvent.deltaMode === 2) return delta * (axis === 'horizontal' ? scrollElement.clientWidth : scrollElement.clientHeight);
		return delta;
	}

	function currentOffset(scrollElement: HTMLElement): number {
		return axis === 'horizontal' ? scrollElement.scrollLeft : scrollElement.scrollTop;
	}

	function applyOffset(scrollElement: HTMLElement, offset: number): void {
		if (axis === 'horizontal') scrollElement.scrollLeft = offset;
		else scrollElement.scrollTop = offset;
	}

	function clearSettledOffset(): void {
		settledOffset = null;
	}

	function handleWheel(wheelEvent: WheelEvent): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement || wheelEvent.ctrlKey) return;
		wheelEvent.preventDefault();
		clearSettledOffset();

		const magnitude = Math.abs(wheelDelta(wheelEvent));
		const now = performance.now();
		const isNewGesture = now - previousWheelTime > gestureGapMilliseconds;
		const isAcceleratingAgain = magnitude > previousMagnitude + 0.5;
		previousWheelTime = now;

		if (isIdleSnapped) {
			isIdleSnapped = false;
			isIgnoringMomentum = false;
			decayingSteps = 0;
			gestureStartOffset = currentOffset(scrollElement);
			gestureStartTime = now;
			gestureLiftTime = now;
			gestureLiftOffset = gestureStartOffset;
		}
		if (isNewGesture) {
			isIgnoringMomentum = false;
			decayingSteps = 0;
			gestureStartOffset = currentOffset(scrollElement);
			gestureStartTime = now;
			gestureLiftTime = now;
			gestureLiftOffset = gestureStartOffset;
			cancelAnimation();
		} else if (isAcceleratingAgain && animationFrame === null) {
			isIgnoringMomentum = false;
			decayingSteps = 0;
		}
		if (isIgnoringMomentum) {
			previousMagnitude = magnitude;
			return;
		}

		queueWheelScroll(scrollElement, wheelDeltaPixels(wheelEvent, scrollElement), isNewGesture);
		scheduleIdleSnap();
		const isDecaying = magnitude < previousMagnitude - 0.5;
		decayingSteps = isDecaying ? decayingSteps + 1 : 0;
		if (!isDecaying) {
			gestureLiftTime = now;
			gestureLiftOffset = wheelTargetOffset;
		}
		previousMagnitude = magnitude;
		if (decayingSteps < momentumDecaySteps) return;
		isIgnoringMomentum = true;
		snapToNearestOffset();
	}

	function scheduleIdleSnap(): void {
		if (idleSnapTimer !== null) clearTimeout(idleSnapTimer);
		idleSnapTimer = window.setTimeout(() => {
			idleSnapTimer = null;
			if (isIgnoringMomentum) return;
			isIgnoringMomentum = true;
			isIdleSnapped = true;
			snapToNearestOffset();
		}, idleSnapMilliseconds);
	}

	function queueWheelScroll(scrollElement: HTMLElement, deltaPixels: number, isNewGesture: boolean): void {
		const maximumOffset =
			axis === 'horizontal'
				? scrollElement.scrollWidth - scrollElement.clientWidth
				: scrollElement.scrollHeight - scrollElement.clientHeight;
		const baseOffset = isNewGesture || wheelFrame === null ? currentOffset(scrollElement) : wheelTargetOffset;
		wheelTargetOffset = Math.max(0, Math.min(maximumOffset, baseOffset + deltaPixels));
		if (wheelFrame !== null) return;
		wheelFrame = requestAnimationFrame(stepWheelScroll);
	}

	function stepWheelScroll(): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement) {
			wheelFrame = null;
			return;
		}
		const offset = currentOffset(scrollElement);
		const remainingDistance = wheelTargetOffset - offset;
		if (Math.abs(remainingDistance) < 0.5) {
			applyOffset(scrollElement, wheelTargetOffset);
			wheelFrame = null;
			return;
		}
		applyOffset(scrollElement, offset + remainingDistance * wheelFollowFactor);
		wheelFrame = requestAnimationFrame(stepWheelScroll);
	}

	function cancelWheelScroll(): void {
		if (wheelFrame === null) return;
		cancelAnimationFrame(wheelFrame);
		wheelFrame = null;
	}

	function handleTouchStart(): void {
		clearSettledOffset();
	}

	function handleGestureEnd(): void {
		gestureLiftTime = performance.now();
		gestureLiftOffset = wheelTargetOffset;
		isIgnoringMomentum = true;
		snapToNearestOffset();
	}

	function handleScrollEnd(): void {
		if (animationFrame !== null) return;
		const scrollElement = options.getScrollElement();
		if (scrollElement && settledOffset !== null && Math.abs(currentOffset(scrollElement) - settledOffset) < 1) return;
		snapToNearestOffset();
	}

	function snapToNearestOffset(): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement) return;
		const offset = wheelFrame !== null ? wheelTargetOffset : currentOffset(scrollElement);
		cancelWheelScroll();
		const offsets = options.getSnapOffsets();
		if (offsets.length === 0) return;
		const nearestOffset = options.resolveSnapOffset
			? options.resolveSnapOffset({
					currentOffset: offset,
					gestureStartOffset,
					offsets,
					gestureVelocity: gestureVelocityPixelsPerMillisecond()
				})
			: nearestSnapOffset(offset, offsets);
		cancelWheelScroll();
		if (Math.abs(nearestOffset - offset) < 1) {
			settledOffset = nearestOffset;
			options.onSnapSettled?.(nearestOffset);
			return;
		}
		animateScrollTo(nearestOffset);
	}

	function gestureVelocityPixelsPerMillisecond(): number {
		const elapsedMilliseconds = gestureLiftTime - gestureStartTime;
		if (elapsedMilliseconds <= 0) return 0;
		return Math.abs(gestureLiftOffset - gestureStartOffset) / elapsedMilliseconds;
	}

	function nearestSnapOffset(offset: number, offsets: number[]): number {
		return offsets.reduce(
			(closest, candidate) => (Math.abs(candidate - offset) < Math.abs(closest - offset) ? candidate : closest),
			offsets[0]
		);
	}

	function animateScrollTo(targetScrollTop: number): void {
		const scrollElement = options.getScrollElement();
		if (!scrollElement) return;
		cancelAnimation();
		const startOffset = currentOffset(scrollElement);
		const scrollDistance = targetScrollTop - startOffset;
		const startTime = performance.now();
		const durationMilliseconds = Math.min(
			maximumSnapMilliseconds,
			Math.max(animationMilliseconds, Math.abs(scrollDistance) * millisecondsPerSnapPixel)
		);
		const step = (now: number) => {
			const currentScrollElement = options.getScrollElement();
			if (!currentScrollElement) return;
			const progress = Math.min(1, (now - startTime) / durationMilliseconds);
			applyOffset(currentScrollElement, startOffset + scrollDistance * (1 - (1 - progress) ** 3));
			if (progress < 1) {
				animationFrame = requestAnimationFrame(step);
				return;
			}
			animationFrame = null;
			settledOffset = targetScrollTop;
			options.onSnapSettled?.(targetScrollTop);
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
		handleTouchStart,
		destroy: () => {
			if (idleSnapTimer !== null) clearTimeout(idleSnapTimer);
			cancelWheelScroll();
			cancelAnimation();
		}
	};
}

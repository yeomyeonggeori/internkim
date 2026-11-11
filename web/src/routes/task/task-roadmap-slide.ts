const slideTiming: KeyframeAnimationOptions = { duration: 320, easing: 'cubic-bezier(0.22, 1, 0.36, 1)' };

export function slideTaskRoadmap(root: HTMLElement, movedDays: number, visibleDayCount: number, laneWidth: number): void {
	if (movedDays === 0 || visibleDayCount === 0) return;
	const share = Math.max(-1, Math.min(1, movedDays / visibleDayCount));
	const from = `translateX(${share * laneWidth}px)`;
	for (const element of root.querySelectorAll('[data-roadmap-slide="move"]')) {
		element.animate([{ transform: from, opacity: 0.2 }, { transform: 'none', opacity: 1 }], slideTiming);
	}
	for (const element of root.querySelectorAll('[data-roadmap-slide="fade"]')) {
		element.animate([{ opacity: 0 }, { opacity: 1 }], slideTiming);
	}
}

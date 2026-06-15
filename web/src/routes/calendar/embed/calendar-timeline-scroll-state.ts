// 캘린더 day/week timeline 스크롤 상태를 stage class로 동기화합니다.
const timelineScrollerSelector = '.df-week-time-grid-scroller, .df-day-content-grid, .df-day-content-grid-rows';
const scrolledClassName = 'calendar-stage-scrolled';

export function installCalendarTimelineScrollState(stageElement: HTMLElement): () => void {
	const scrollers = new Set<HTMLElement>();

	const syncScrolledState = () => {
		const isScrolled = Array.from(scrollers).some((scroller) => scroller.scrollTop > 0);
		stageElement.classList.toggle(scrolledClassName, isScrolled);
	};

	const bindScroller = (scroller: HTMLElement) => {
		if (scrollers.has(scroller)) return;
		scrollers.add(scroller);
		scroller.addEventListener('scroll', syncScrolledState, { passive: true });
	};

	const refreshScrollers = () => {
		for (const scroller of stageElement.querySelectorAll<HTMLElement>(timelineScrollerSelector)) {
			bindScroller(scroller);
		}
		syncScrolledState();
	};

	const observer = new MutationObserver(refreshScrollers);
	observer.observe(stageElement, { childList: true, subtree: true });
	refreshScrollers();

	return () => {
		observer.disconnect();
		for (const scroller of scrollers) {
			scroller.removeEventListener('scroll', syncScrolledState);
		}
		stageElement.classList.remove(scrolledClassName);
	};
}

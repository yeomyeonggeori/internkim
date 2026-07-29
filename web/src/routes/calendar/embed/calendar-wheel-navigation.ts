// 캘린더 월 뷰의 자유 스크롤과 주 단위 스냅을 담당합니다.
import { ViewType } from '@dayflow/svelte';

export type CalendarWheelNavigationOptions = {
	stageElement: HTMLElement;
	currentView: () => string;
	selectVisibleDate: (date: Date) => void;
};

const scrollIdleMs = 140;

export function installCalendarWheelNavigation(options: CalendarWheelNavigationOptions): () => void {
	let scroller: HTMLElement | null = null;
	let lastScrollTop = 0;
	let snapTimer: number | null = null;
	let animationFrame: number | null = null;

	const scheduleConnect = () => {
		if (animationFrame) window.cancelAnimationFrame(animationFrame);
		animationFrame = window.requestAnimationFrame(() => {
			animationFrame = null;
			connectScroller();
		});
	};

	const connectScroller = () => {
		const nextScroller = options.stageElement.querySelector<HTMLElement>('.df-month-view-virtual-scroller');
		if (!nextScroller) {
			scroller?.removeEventListener('scroll', onScroll);
			scroller = null;
			return;
		}
		if (nextScroller === scroller) return;
		scroller?.removeEventListener('scroll', onScroll);
		scroller = nextScroller;
		lastScrollTop = scroller.scrollTop;
		scroller.addEventListener('scroll', onScroll, { passive: true });
	};

	const observer = new MutationObserver(scheduleConnect);
	observer.observe(options.stageElement, { childList: true, subtree: true });
	scheduleConnect();

	const onScroll = () => {
		if (!scroller || options.currentView() !== ViewType.MONTH) return;
		const direction: 1 | -1 = scroller.scrollTop >= lastScrollTop ? 1 : -1;
		lastScrollTop = scroller.scrollTop;
		if (snapTimer) window.clearTimeout(snapTimer);
		snapTimer = window.setTimeout(() => {
			snapTimer = null;
			snapToNearestWeek();
		}, scrollIdleMs);
	};

	const closestVisibleMonthStartCell = () => {
		const stageRectangle = options.stageElement.getBoundingClientRect();
		return (
			visibleMonthStartCells().sort((firstCell, secondCell) => {
				const firstDistance = Math.abs(firstCell.getBoundingClientRect().top - stageRectangle.top);
				const secondDistance = Math.abs(secondCell.getBoundingClientRect().top - stageRectangle.top);
				return firstDistance - secondDistance;
			})[0] ?? null
		);
	};

	const visibleMonthStartCells = () => {
		const stageRectangle = options.stageElement.getBoundingClientRect();
		const monthStartCells = Array.from(options.stageElement.querySelectorAll<HTMLElement>('.df-month-day-cell[data-date$="-01"]'));
		return monthStartCells.filter((cell) => {
			const rectangle = cell.getBoundingClientRect();
			return rectangle.bottom >= stageRectangle.top && rectangle.top <= stageRectangle.bottom;
		});
	};

	const snapToNearestWeek = () => {
		if (!scroller || options.currentView() !== ViewType.MONTH) return;
		const targetWeek = nearestWeekToScrollerTop();
		if (!targetWeek) return;
		const scrollerRectangle = scroller.getBoundingClientRect();
		const weekRectangle = targetWeek.getBoundingClientRect();
		const scrollOffset = weekRectangle.top - scrollerRectangle.top;
		scroller.scrollBy({ top: scrollOffset, behavior: 'smooth' });
		const targetDateCell = closestVisibleMonthStartCell() ?? targetWeek.querySelector<HTMLElement>('.df-month-day-cell[data-date]');
		const targetDate = targetDateCell ? dateFromCell(targetDateCell) : null;
		if (targetDate) options.selectVisibleDate(targetDate);
	};

	const nearestWeekToScrollerTop = () => {
		if (!scroller) return null;
		const scrollerRectangle = scroller.getBoundingClientRect();
		const weekElements = Array.from(options.stageElement.querySelectorAll<HTMLElement>('.df-month-week'));
		return (
			weekElements
				.filter((weekElement) => {
					const rectangle = weekElement.getBoundingClientRect();
					return rectangle.bottom >= scrollerRectangle.top && rectangle.top <= scrollerRectangle.bottom;
				})
				.sort((firstWeek, secondWeek) => {
					const firstDistance = Math.abs(firstWeek.getBoundingClientRect().top - scrollerRectangle.top);
					const secondDistance = Math.abs(secondWeek.getBoundingClientRect().top - scrollerRectangle.top);
					return firstDistance - secondDistance;
				})[0] ?? null
		);
	};

	const dateFromCell = (dateCell: HTMLElement) => {
		const value = dateCell.dataset.date;
		if (!value) return null;
		const [year = '0', month = '1', day = '1'] = value.split('-');
		return new Date(Number(year), Number(month) - 1, Number(day), 12, 0, 0, 0);
	};

	return () => {
		observer.disconnect();
		if (animationFrame) window.cancelAnimationFrame(animationFrame);
		if (snapTimer) window.clearTimeout(snapTimer);
		scroller?.removeEventListener('scroll', onScroll);
	};
}

// 캘린더 embed 페이지의 DayFlow DOM 후처리 예약을 담당합니다.
import type { CalendarViewType } from '@dayflow/core';
import { syncCalendarAllDayLayout } from './calendar-all-day-layout';
import {
	enhanceDayFlowMiniCalendar,
	type DayFlowMiniCalendarEnhancementContext
} from './calendar-dayflow-mini-calendar-enhancement';

type CalendarDOMSyncTask = () => void;

export function scheduleDayFlowMiniCalendarEnhancement(context: DayFlowMiniCalendarEnhancementContext): void {
	scheduleCalendarDOMSync(() => enhanceDayFlowMiniCalendar(context));
}

export function scheduleCalendarAllDayLayoutSync(stageElement: HTMLElement | null, currentView: CalendarViewType): void {
	scheduleCalendarDOMSync(() => syncCalendarAllDayLayout(stageElement, currentView));
}

function scheduleCalendarDOMSync(task: CalendarDOMSyncTask): void {
	if (typeof requestAnimationFrame === 'undefined') return;
	requestAnimationFrame(task);
	requestAnimationFrame(() => requestAnimationFrame(task));
}

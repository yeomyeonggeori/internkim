// 출결 팀 상세 팝업에 필요한 외부 컨텍스트 데이터를 불러온다.
import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
import { fetchCalendarEvents } from '../../calendar/embed/calendar-event-persistence';
import { fetchFlowState } from '../../flow/flow-api';
import type { FlowState } from '../../flow/flow-types';
import { addDays } from '../shared/attendance-date';

export type TeamStatusDayContextData = {
	calendarEvents: CalendarEvent[];
	flowState: FlowState | null;
	hasCalendarEventsLoadFailed: boolean;
	hasCompletedWorkLoadFailed: boolean;
};

export async function loadTeamStatusDayContextData(
	firstDate: string,
	lastDate: string,
	fallbackMessage: string
): Promise<TeamStatusDayContextData> {
	const startDate = dateFromKey(firstDate);
	const endDate = dateFromKey(addDays(lastDate, 1));
	const [calendarResult, flowResult] = await Promise.allSettled([
		fetchCalendarEvents(startDate, endDate, fallbackMessage),
		fetchFlowState(fallbackMessage)
	]);
	return {
		calendarEvents: calendarResult.status === 'fulfilled' ? calendarResult.value : [],
		flowState: flowResult.status === 'fulfilled' ? flowResult.value : null,
		hasCalendarEventsLoadFailed: calendarResult.status === 'rejected',
		hasCompletedWorkLoadFailed: flowResult.status === 'rejected'
	};
}

function dateFromKey(date: string): Date {
	return new Date(`${date}T00:00:00`);
}

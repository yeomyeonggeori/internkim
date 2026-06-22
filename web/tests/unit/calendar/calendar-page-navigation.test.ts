import { ViewType } from '@dayflow/svelte';
import { describe, expect, test } from 'bun:test';

import { createCalendarPageNavigation } from '../../../src/routes/calendar/embed/calendar-page-navigation';

describe('calendar page navigation', () => {
	test('selects the searched event after moving the calendar to the event date', () => {
		const calendarDateActions: string[] = [];
		const visibleDates: Date[] = [];
		const selectedEventIDs: string[] = [];
		const selectedMonthDateKeys: string[] = [];

		const navigation = createCalendarPageNavigation({
			calendar: {
				changeView: () => {},
				goToToday: () => {},
				goToPrevious: () => {},
				goToNext: () => {},
				app: {
					setCurrentDate: (date) => calendarDateActions.push(`current:${date.toISOString()}`),
					setVisibleMonth: (date) => calendarDateActions.push(`month:${date.toISOString()}`),
					selectDate: (date) => calendarDateActions.push(`select:${date.toISOString()}`)
				}
			},
			getToolbarDate: () => new Date(2026, 5, 1),
			getToolbarView: () => ViewType.MONTH,
			setToolbarView: () => {},
			setVisibleDate: (date) => visibleDates.push(date),
			setSelectedMonthDateKey: (dateKey) => selectedMonthDateKeys.push(dateKey),
			refreshSelectedMonthDateCellAfterRender: () => {},
			selectCalendarEvent: (eventID) => selectedEventIDs.push(eventID),
			broadcastCalendarView: () => {}
		});

		const eventDate = new Date(2026, 5, 4, 18, 0);
		navigation.navigateToSearchResult({
			id: 'event-1',
			title: '디플랫코리아 기획안 전달',
			startDate: eventDate,
			dateLabel: '2026.06.04 THU',
			highlightParts: [{ text: '디플랫코리아 기획안 전달', isMatch: false }]
		});

		expect(visibleDates).toEqual([eventDate]);
		expect(calendarDateActions).toEqual([
			`current:${eventDate.toISOString()}`,
			`month:${eventDate.toISOString()}`,
			`select:${eventDate.toISOString()}`
		]);
		expect(selectedEventIDs).toEqual(['event-1']);
		expect(selectedMonthDateKeys).toEqual([]);
	});
});

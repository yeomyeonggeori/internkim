import { ViewType } from '@dayflow/svelte';
import { describe, expect, test } from 'bun:test';

import { createCalendarPageNavigation } from '../../../src/routes/calendar/embed/calendar-page-navigation';
import { shiftedCalendarToolbarDate } from '../../../src/routes/calendar/embed/calendar-visible-range';

describe('calendar page navigation', () => {
	test('selects the searched event after moving the calendar to the event date', () => {
		const visibleDates: Date[] = [];
		const selectedEventIDs: string[] = [];
		const selectedMonthDateKeys: string[] = [];

		const navigation = createCalendarPageNavigation({
			getToolbarDate: () => new Date(2026, 5, 1),
			getToolbarView: () => ViewType.MONTH,
			setToolbarView: () => {},
			setVisibleDate: (date) => visibleDates.push(date),
			setSelectedMonthDateKey: (dateKey) => selectedMonthDateKeys.push(dateKey),
			selectCalendarEvent: (eventID) => selectedEventIDs.push(eventID),
			broadcastCalendarView: () => {},
			isMobileTwoDayWeekView: () => false
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
		expect(selectedEventIDs).toEqual(['event-1']);
		expect(selectedMonthDateKeys).toEqual([]);
	});

	test('moves mobile week navigation by two days', () => {
		const nextDate = shiftedCalendarToolbarDate(new Date(2026, 5, 4), ViewType.WEEK, 1, true);
		const previousDate = shiftedCalendarToolbarDate(new Date(2026, 5, 4), ViewType.WEEK, -1, true);

		expect(nextDate).toEqual(new Date(2026, 5, 6));
		expect(previousDate).toEqual(new Date(2026, 5, 2));
	});

	test('keeps desktop week navigation on seven day increments', () => {
		const nextDate = shiftedCalendarToolbarDate(new Date(2026, 5, 4), ViewType.WEEK, 1, false);
		const previousDate = shiftedCalendarToolbarDate(new Date(2026, 5, 4), ViewType.WEEK, -1, false);

		expect(nextDate).toEqual(new Date(2026, 5, 11));
		expect(previousDate).toEqual(new Date(2026, 4, 28));
	});
});

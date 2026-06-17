import { createEvent } from '@dayflow/core';
import { describe, expect, test } from 'bun:test';

import {
	monthEventSegments,
	moveMonthEventToDate,
	type MonthEventWeek
} from '../../../src/routes/calendar/embed/calendar-month-event-model';
import { eventEndDate, eventStartDate } from '../../../src/routes/calendar/embed/calendar-event-mapping';

const juneSecondWeek: MonthEventWeek = {
	id: '2026-06-07',
	dateKeys: ['2026-06-07', '2026-06-08', '2026-06-09', '2026-06-10', '2026-06-11', '2026-06-12', '2026-06-13']
};

const juneThirdWeek: MonthEventWeek = {
	id: '2026-06-14',
	dateKeys: ['2026-06-14', '2026-06-15', '2026-06-16', '2026-06-17', '2026-06-18', '2026-06-19', '2026-06-20']
};

describe('monthEventSegments', () => {
	test('clips a timed multi-day event to each visible week and includes start and end times', () => {
		const event = createEvent({
			id: 'timed-cross-week',
			title: 'Timed Cross Week',
			start: new Date(2026, 5, 12, 9, 30),
			end: new Date(2026, 5, 16, 10, 45),
			calendarId: 'internkim'
		});

		expect(monthEventSegments([event], [juneSecondWeek, juneThirdWeek])).toMatchObject([
			{
				eventID: 'timed-cross-week',
				startDateKey: '2026-06-12',
				endDateKey: '2026-06-13',
				titleText: 'Timed Cross Week 09:30',
				endTimeText: '10:45',
				lane: 0
			},
			{
				eventID: 'timed-cross-week',
				startDateKey: '2026-06-14',
				endDateKey: '2026-06-16',
				titleText: 'Timed Cross Week 09:30',
				endTimeText: '10:45',
				lane: 0
			}
		]);
	});

	test('treats a midnight timed end as the previous display date', () => {
		const event = createEvent({
			id: 'single-24h',
			title: 'Single 24h',
			start: new Date(2026, 5, 10, 0, 0),
			end: new Date(2026, 5, 11, 0, 0),
			calendarId: 'internkim'
		});

		expect(monthEventSegments([event], [juneSecondWeek])).toMatchObject([
			{
				eventID: 'single-24h',
				startDateKey: '2026-06-10',
				endDateKey: '2026-06-10',
				titleText: 'Single 24h 00:00',
				endTimeText: '00:00'
			}
		]);
	});

	test('places overlapping events on separate lanes in a week', () => {
		const firstEvent = createEvent({
			id: 'first-event',
			title: 'First Event',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 12),
			allDay: true,
			calendarId: 'internkim'
		});
		const secondEvent = createEvent({
			id: 'second-event',
			title: 'Second Event',
			start: new Date(2026, 5, 11, 9),
			end: new Date(2026, 5, 11, 10),
			calendarId: 'internkim'
		});

		expect(monthEventSegments([firstEvent, secondEvent], [juneSecondWeek]).map((segment) => [segment.eventID, segment.lane])).toEqual([
			['first-event', 0],
			['second-event', 1]
		]);
	});

	test('places longer overlapping events above shorter events', () => {
		const shortEvent = createEvent({
			id: 'short-event',
			title: 'Short Event',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 10),
			allDay: true,
			calendarId: 'internkim'
		});
		const longEvent = createEvent({
			id: 'long-event',
			title: 'Long Event',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 12),
			allDay: true,
			calendarId: 'internkim'
		});

		expect(monthEventSegments([shortEvent, longEvent], [juneSecondWeek]).map((segment) => [segment.eventID, segment.lane])).toEqual([
			['long-event', 0],
			['short-event', 1]
		]);
	});

	test('places recently updated equal-length events and local drafts above older events', () => {
		const olderEvent = createEvent({
			id: 'z-older-event',
			title: 'Older Event',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 11),
			allDay: true,
			calendarId: 'internkim',
			meta: { updatedAt: '2026-06-01T00:00:00.000Z' }
		});
		const newerEvent = createEvent({
			id: 'a-newer-event',
			title: 'Newer Event',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 11),
			allDay: true,
			calendarId: 'internkim',
			meta: { updatedAt: '2026-06-02T00:00:00.000Z' }
		});
		const persistedWithoutMetadata = createEvent({
			id: 'persisted-without-metadata',
			title: 'Persisted Without Metadata',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 11),
			allDay: true,
			calendarId: 'internkim'
		});
		const draftWithoutMetadata = createEvent({
			id: 'draft-without-metadata',
			title: 'Draft Without Metadata',
			start: new Date(2026, 5, 10),
			end: new Date(2026, 5, 11),
			allDay: true,
			calendarId: 'internkim',
			meta: { localSortAt: '2026-06-17T00:00:00.000Z' }
		});

		expect(
			monthEventSegments([olderEvent, newerEvent, persistedWithoutMetadata, draftWithoutMetadata], [juneSecondWeek]).map(
				(segment) => [segment.eventID, segment.lane]
			)
		).toEqual([
			['draft-without-metadata', 0],
			['a-newer-event', 1],
			['z-older-event', 2],
			['persisted-without-metadata', 3]
		]);
	});

	test('keeps a timed draft with an empty title visible in its month week', () => {
		const event = createEvent({
			id: 'empty-title-draft',
			title: '',
			start: new Date(2026, 5, 20, 9, 0),
			end: new Date(2026, 5, 20, 10, 0),
			calendarId: 'internkim'
		});

		expect(monthEventSegments([event], [juneThirdWeek])).toMatchObject([
			{
				eventID: 'empty-title-draft',
				startDateKey: '2026-06-20',
				endDateKey: '2026-06-20',
				titleText: '09:00',
				endTimeText: '10:00'
			}
		]);
	});
});

describe('moveMonthEventToDate', () => {
	test('moves a timed multi-day event while preserving duration and times', () => {
		const event = createEvent({
			id: 'move-timed',
			title: 'Move Timed',
			start: new Date(2026, 5, 10, 9, 0),
			end: new Date(2026, 5, 12, 10, 0),
			calendarId: 'internkim'
		});

		const movedEvent = moveMonthEventToDate(event, '2026-06-11');

		expect(eventStartDate(movedEvent)).toEqual(new Date(2026, 5, 11, 9, 0));
		expect(eventEndDate(movedEvent)).toEqual(new Date(2026, 5, 13, 10, 0));
		expect(movedEvent.title).toBe('Move Timed');
	});

	test('marks moved events as recently updated locally before the server response returns', () => {
		const event = createEvent({
			id: 'move-recent',
			title: 'Move Recent',
			start: new Date(2026, 5, 10, 9, 0),
			end: new Date(2026, 5, 10, 10, 0),
			calendarId: 'internkim',
			meta: { updatedAt: '2026-06-01T00:00:00.000Z', location: 'Seoul' }
		});

		const movedEvent = moveMonthEventToDate(event, '2026-06-11', new Date('2026-06-17T09:30:00.000Z'));

		expect(movedEvent.meta).toMatchObject({
			location: 'Seoul',
			localSortAt: '2026-06-17T09:30:00.000Z',
			updatedAt: '2026-06-17T09:30:00.000Z'
		});
	});
});

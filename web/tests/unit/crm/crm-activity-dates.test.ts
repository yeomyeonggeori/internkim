import { describe, expect, test } from 'bun:test';

import { activityDatesOf, activityDatesTurned, activityDatesWritten } from '../../../src/routes/crm/crm-activity-dates';

describe('CRM activity dates', () => {
	test('an activity held without dates is edited as today, all day', () => {
		const now = new Date('2026-10-21T03:00:00Z');
		const dates = activityDatesOf({ occurredAt: '2026-10-01T00:00:00Z' }, now);
		expect(dates.isWholeDay).toBe(true);
		expect(dates.startsAt).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		expect(dates.endsAt).toBe(dates.startsAt);
	});

	test('a whole-day activity keeps the days the record answered', () => {
		const dates = activityDatesOf({ isWholeDay: true, startsAt: '2026-10-06', endsAt: '2026-10-07', occurredAt: '' });
		expect(dates).toEqual({ isWholeDay: true, startsAt: '2026-10-06', endsAt: '2026-10-07' });
	});

	test('a timed activity turned into whole days keeps its days', () => {
		const turned = activityDatesTurned({ isWholeDay: false, startsAt: '2026-10-06T14:00', endsAt: '2026-10-07T15:00' }, true);
		expect(turned).toEqual({ isWholeDay: true, startsAt: '2026-10-06', endsAt: '2026-10-07' });
	});

	test('whole days turned into a time end an hour after they start on a single day', () => {
		const turned = activityDatesTurned(
			{ isWholeDay: true, startsAt: '2026-10-06', endsAt: '2026-10-06' },
			false,
			new Date(2026, 9, 21, 14, 15)
		);
		expect(turned).toEqual({ isWholeDay: false, startsAt: '2026-10-06T14:15', endsAt: '2026-10-06T15:15' });
	});

	test('whole days go out as days and an end defaults to the start', () => {
		expect(activityDatesWritten({ isWholeDay: true, startsAt: '2026-10-06', endsAt: '' })).toEqual({
			isWholeDay: true,
			startsAt: '2026-10-06',
			endsAt: '2026-10-06'
		});
	});
});

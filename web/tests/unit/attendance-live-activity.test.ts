import { describe, expect, test } from 'bun:test';
import { liveActivityPayload } from '../../../supabase/functions/_shared/apns-live-activity.ts';
import { activityChangesFor } from '../../../supabase/functions/_shared/attendance-live-activity.ts';
import { widgetRefreshPayload } from '../../../supabase/functions/_shared/attendance-widget-refresh.ts';

const alert = { title: '출근', body: '사무실 · 09:02' };
const starter = { kind: 'apns-activity-start', address: 'start-token' };
const running = { kind: 'apns-activity', address: 'activity-token' };

describe('the Live Activity a clock leaves on its own phones', () => {
	test('a clock-in starts one on every phone that can start one', () => {
		const changes = activityChangesFor(
			{ kind: 'clock_in', location: '사무실', occurred_at: '2026-09-17T00:02:00Z' },
			[starter],
			alert
		);
		expect(changes).toEqual([
			{
				device: starter,
				change: { event: 'start', state: { startedAt: 1789603320, location: '사무실' }, alert }
			}
		]);
	});

	test('a clock-in while one is known to run ends it before starting a fresh one, since the person may have cleared it', () => {
		const changes = activityChangesFor(
			{ kind: 'clock_in', location: '재택', occurred_at: '2026-09-17T04:00:00Z' },
			[starter, running],
			alert
		);
		expect(changes.map(({ device, change }) => [device.address, change.event])).toEqual([
			['activity-token', 'end'],
			['start-token', 'start']
		]);
	});

	test('a clock-out ends the running one and never starts anything', () => {
		const changes = activityChangesFor(
			{ kind: 'clock_out', location: null, occurred_at: '2026-09-17T09:00:00Z' },
			[starter, running],
			alert
		);
		expect(changes.map(({ device, change }) => [device.address, change.event])).toEqual([['activity-token', 'end']]);
	});

	test('a clock-out with nothing running asks nothing of any phone', () => {
		expect(
			activityChangesFor({ kind: 'clock_out', location: null, occurred_at: '2026-09-17T09:00:00Z' }, [starter], alert)
		).toEqual([]);
	});
});

describe('the Live Activity payload', () => {
	const state = { startedAt: 1789603320, location: '사무실' };

	test('a start names the attributes the app declared and says why it appeared', () => {
		expect(liveActivityPayload({ event: 'start', state, alert }, 1789603330)).toEqual({
			aps: {
				timestamp: 1789603330,
				event: 'start',
				'content-state': state,
				'attributes-type': 'AttendanceActivityAttributes',
				attributes: {},
				alert
			}
		});
	});

	test('an end dismisses at once', () => {
		expect(liveActivityPayload({ event: 'end', state }, 1789603330)).toEqual({
			aps: { timestamp: 1789603330, event: 'end', 'content-state': state, 'dismissal-date': 1789603330 }
		});
	});
});

describe("the push that refreshes a member's own widgets", () => {
	test('wakes the app without showing anything and says which clock it carries', () => {
		expect(widgetRefreshPayload('clock_out')).toEqual({
			aps: { 'content-available': 1 },
			widget: 'attendance',
			clock: 'clock_out'
		});
	});
});

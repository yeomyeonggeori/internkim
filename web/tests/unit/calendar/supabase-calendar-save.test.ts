import { describe, expect, test } from 'bun:test';
import { supabaseCalendarEventRPCArguments } from '../../../src/lib/calendar/supabase-calendar';
import type { CalendarEventPayload } from '../../../src/routes/calendar/embed/calendar-event-persistence';

function eventWith(fields: Partial<CalendarEventPayload> = {}): CalendarEventPayload {
	return {
		eventID: 'event-1',
		title: '팀 회의',
		description: '주간 진행 공유',
		location: '회의실 A',
		startISO: '2026-08-20T10:00:00.000Z',
		endISO: '2026-08-20T11:00:00.000Z',
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '',
		participants: [
			{ personID: 'member-1', name: '첫 번째' },
			{ personID: 'member-2', name: '두 번째' }
		],
		...fields
	};
}

describe('central calendar event writes', () => {
	test('sends event fields and the complete participant set through one RPC', () => {
		expect(supabaseCalendarEventRPCArguments(eventWith(), 'event-1')).toEqual({
			target_task_id: 'event-1',
			target_title: '팀 회의',
			target_note: '주간 진행 공유',
			target_location: { name: '회의실 A' },
			target_starts_at: '2026-08-20T10:00:00.000Z',
			target_ends_at: '2026-08-20T11:00:00.000Z',
			target_is_whole_day: false,
			target_is_event: true,
			target_size: 'XS',
			target_participant_ids: ['member-1', 'member-2']
		});
	});

	test('a create sends no task ID even though the draft carries a local one', () => {
		const argumentsForCreate = supabaseCalendarEventRPCArguments(
			eventWith({ eventID: 'quick-1788331680143-umwpd4', description: '', location: '' }),
			null
		);

		expect(argumentsForCreate.target_task_id).toBeNull();
		expect(argumentsForCreate.target_note).toBeNull();
		expect(argumentsForCreate.target_location).toBeNull();
	});

	test('an update names the persisted event, not whatever ID the payload carries', () => {
		const argumentsForUpdate = supabaseCalendarEventRPCArguments(
			eventWith({ eventID: 'quick-1788331680143-umwpd4' }),
			'2f1c6b1e-0000-4000-8000-000000000001'
		);

		expect(argumentsForUpdate.target_task_id).toBe('2f1c6b1e-0000-4000-8000-000000000001');
	});
});

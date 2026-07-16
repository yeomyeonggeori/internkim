import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { buildAttendanceSummaryFixture } from '../../../dev-attendance-summary-fixture';
import type {
	AttendanceEvent,
	AttendanceSummary
} from '../../../src/routes/attendance/attendance-context.svelte';
import type { TeamStatusPersonDaySegment } from '../../../src/routes/attendance/team/team-status-table-model';
import type { WorkRecordUpdate } from '../../../src/routes/attendance/team/work-record-editor.svelte';

type WorkRecordEditorModule = typeof import(
	'../../../src/routes/attendance/team/work-record-editor.svelte'
);

const serverTime = new Date('2026-07-15T10:30:00+09:00');
const localDate = '2026-07-15';
const segment: TeamStatusPersonDaySegment = {
	id: 'segment-1',
	startEventID: 'clock-in',
	endEventID: 'clock-out',
	endReason: 'clock_out',
	locationName: '사무실',
	locationID: 'office',
	locationColor: '#22c55e',
	startTime: '09:00',
	endTime: '10:00',
	durationMinutes: 60,
	widthPercent: 12.5,
	isOpen: false
};

let originalState: unknown;
let WorkRecordEditorState: WorkRecordEditorModule['WorkRecordEditorState'];

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	({ WorkRecordEditorState } = await import(
		'../../../src/routes/attendance/team/work-record-editor.svelte'
	));
});

afterAll(() => {
	if (originalState === undefined) {
		Reflect.deleteProperty(globalThis, '$state');
	} else {
		Reflect.set(globalThis, '$state', originalState);
	}
});

describe('work record editor state', () => {
	test('opens an editing session with drafts for the displayed work segment', () => {
		const fixture = createEditorFixture();

		expect(fixture.editor.open([segment])).toBe(true);
		expect(fixture.editor.isEditing).toBe(true);
		expect(fixture.editor.draftFor(segment.startEventID)).toEqual({
			eventID: 'clock-in',
			localDate,
			originalLocalTime: '09:00',
			localTime: '09:00',
			originalLocationID: 'office',
			locationID: 'office'
		});
	});

	test('falls future input back to the current server minute', () => {
		const fixture = createEditorFixture();
		fixture.editor.open([segment]);

		expect(fixture.editor.updateEventTime(segment.endEventID, '23:59')).toBe('10:30');
		expect(fixture.editor.maximumTimeFor(localDate)).toBe('10:30');
	});

	test('updates both clock events when a closed segment location changes', () => {
		const fixture = createEditorFixture();
		fixture.editor.open([segment]);

		fixture.editor.updateSegmentLocation(segment, 'remote');

		expect(fixture.editor.draftFor(segment.startEventID)?.locationID).toBe('remote');
		expect(fixture.editor.draftFor(segment.endEventID)?.locationID).toBe('remote');
	});

	test('saves only changed events with a trimmed reason', async () => {
		const fixture = createEditorFixture();
		fixture.editor.open([segment]);
		fixture.editor.updateEventTime(segment.endEventID, '10:15');
		fixture.editor.reason = '  corrected time  ';

		await fixture.editor.save();

		expect(fixture.updates).toEqual([
			[
				{
					eventID: 'clock-out',
					request: {
						localDate,
						localTime: '10:15',
						locationID: 'office',
						reason: 'corrected time'
					}
				}
			]
		]);
		expect(fixture.editor.isEditing).toBe(false);
	});

	test('refuses unavailable server time and detects a time zone change', () => {
		const fixture = createEditorFixture();
		fixture.summary.timeZoneAuthoritative = false;
		expect(fixture.editor.open([segment])).toBe(false);

		fixture.summary.timeZoneAuthoritative = true;
		expect(fixture.editor.open([segment])).toBe(true);
		fixture.summary.timeZone = 'America/Los_Angeles';
		expect(fixture.editor.matchesTimeZone(fixture.summary.timeZone)).toBe(false);
	});

	test('keeps the editing session and localized error when saving fails', async () => {
		const fixture = createEditorFixture();
		fixture.updateError = new Error('server unavailable');
		fixture.editor.open([segment]);
		fixture.editor.updateEventTime(segment.endEventID, '10:15');
		fixture.editor.reason = 'corrected time';

		await fixture.editor.save();

		expect(fixture.editor.isEditing).toBe(true);
		expect(fixture.editor.isSaving).toBe(false);
		expect(fixture.editor.errorMessage).toBe('처리하지 못했습니다.');
	});
});

function createEditorFixture() {
	const summary = createSummary();
	const updates: WorkRecordUpdate[][] = [];
	const fixture = {
		summary,
		hasServerClock: true,
		currentServerTime: serverTime,
		updateError: null as Error | null,
		updates
	};
	const editor = new WorkRecordEditorState({
		getSummary: () => fixture.summary,
		hasServerClock: () => fixture.hasServerClock,
		getCurrentServerTime: () => fixture.currentServerTime,
		updateEvents: async (nextUpdates) => {
			if (fixture.updateError) throw fixture.updateError;
			fixture.updates.push(nextUpdates);
		},
		processingFailedMessage: '처리하지 못했습니다.'
	});
	return {
		summary: fixture.summary,
		hasServerClock: fixture.hasServerClock,
		currentServerTime: fixture.currentServerTime,
		updates: fixture.updates,
		editor,
		get updateError() {
			return fixture.updateError;
		},
		set updateError(error: Error | null) {
			fixture.updateError = error;
		}
	};
}

function createSummary(): AttendanceSummary {
	return {
		...buildAttendanceSummaryFixture('2026-07', serverTime),
		serverTime: serverTime.toISOString(),
		timeZone: 'Asia/Seoul',
		timeZoneAuthoritative: true,
		events: [
			createEvent('clock-in', 'clock_in', '09:00'),
			createEvent('clock-out', 'clock_out', '10:00')
		]
	};
}

function createEvent(
	id: string,
	kind: AttendanceEvent['kind'],
	localTime: string
): AttendanceEvent {
	return {
		id,
		mattermostUserID: 'kim',
		mattermostUsername: 'kim',
		email: 'kim@example.com',
		displayName: '김철수',
		kind,
		occurredAt: `${localDate}T${localTime}:00+09:00`,
		localDate,
		localTime,
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'mattermost_button',
		resultPostID: `${id}-post`,
		locationID: 'office',
		locationName: '사무실'
	};
}

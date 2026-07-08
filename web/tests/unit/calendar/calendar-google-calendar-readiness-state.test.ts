import { describe, expect, test } from 'bun:test';
import {
	googleCalendarConnectionStateClass,
	googleCalendarConnectionStateLabel,
	googleCalendarReadinessStatus
} from '../../../src/routes/calendar/calendar-google-calendar-readiness-state';
import type { CalendarAccountStatusResponse } from '../../../src/routes/calendar/calendar-layout-types';
import { calendarText } from '../../../src/routes/calendar/text';

function connectedCalendarStatus(
	calendarReadinessStatus: CalendarAccountStatusResponse['calendarReadinessStatus']
): CalendarAccountStatusResponse {
	return {
		connected: true,
		needsReauth: false,
		needsCalendarSelection: false,
		initialSyncCompleted: false,
		calendarSyncReady: false,
		calendarReadinessStatus,
		googleOAuthConfigured: true,
		canManageGoogleOAuth: true
	};
}

describe('calendar google calendar readiness state', () => {
	test('labels selected-calendar initial export separately from initial sync', () => {
		const status = connectedCalendarStatus('initial_export_pending');

		expect(googleCalendarReadinessStatus(status)).toBe('initial_export_pending');
		expect(googleCalendarConnectionStateLabel(calendarText.ko, status, false, false)).toBe('초기 내보내기 중');
		expect(googleCalendarConnectionStateLabel(calendarText.en, status, false, false)).toBe('Initial export pending');
		expect(googleCalendarConnectionStateClass(status).includes('text-amber-700')).toBe(true);
	});
});

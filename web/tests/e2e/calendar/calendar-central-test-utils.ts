import type { Page } from '@playwright/test';
import { centralPlaneAdminClient, exampleCompanyID, member1ID, exampleCompanySlug } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';

export type CalendarCentralEvent = {
	id?: string;
	title: string;
	startISO: string;
	endISO: string;
	isAllDay?: boolean;
	note?: string;
	participantIDs?: string[];
};

const calendarPath = `/${exampleCompanySlug}/calendar`;
export const calendarEmbedPath = `${calendarPath}/embed`;

export async function signInToCalendar(page: Page): Promise<void> {
	if (!(await signInToTheCentralPlane(page, calendarPath))) return;
	await page.waitForURL(`**${calendarPath}**`);
	await page.locator('.calendar-stage').waitFor({ state: 'visible' });
}

export async function seedCalendarEvents(events: CalendarCentralEvent[]): Promise<string[]> {
	const admin = centralPlaneAdminClient();
	const rows = events.map((event) => ({
		...(event.id ? { id: event.id } : {}),
		company_id: exampleCompanyID,
		title: event.title,
		is_event: true,
		is_whole_day: event.isAllDay ?? false,
		starts_at: event.startISO,
		ends_at: event.endISO,
		note: event.note ?? '',
		requester_id: member1ID
	}));
	const inserted = await admin.from('task').insert(rows).select('id');
	if (inserted.error) throw new Error(`Failed to seed calendar events: ${inserted.error.message}`);
	const eventIDs = inserted.data.map((row) => row.id as string);
	const participantRows = events.flatMap((event, index) =>
		(event.participantIDs ?? []).map((memberID) => ({ task_id: eventIDs[index], member_id: memberID }))
	);
	if (participantRows.length > 0) {
		const participants = await admin.from('task_participant').insert(participantRows);
		if (participants.error) {
			await cleanupCalendarEvents(eventIDs);
			throw new Error(`Failed to seed calendar event participants: ${participants.error.message}`);
		}
	}
	return eventIDs;
}

export type CalendarEventRow = {
	id: string;
	starts_at: string;
	ends_at: string;
	is_whole_day: boolean;
};

export async function calendarEventRowsTitled(title: string): Promise<CalendarEventRow[]> {
	const admin = centralPlaneAdminClient();
	const found = await admin
		.from('task')
		.select('id, starts_at, ends_at, is_whole_day')
		.eq('company_id', exampleCompanyID)
		.eq('title', title)
		.returns<CalendarEventRow[]>();
	if (found.error) throw new Error(`Failed to read calendar events titled ${title}: ${found.error.message}`);
	return found.data;
}

export async function cleanupCalendarEvents(eventIDs: string[]): Promise<void> {
	if (eventIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('task').delete().in('id', eventIDs);
	if (deleted.error) throw new Error(`Failed to clean up calendar events: ${deleted.error.message}`);
}

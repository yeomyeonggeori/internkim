import type { Page } from '@playwright/test';
import { centralPlaneAdminClient, exampleCompanyID, member1ID } from './central-test-utils';

export type CalendarCentralEvent = {
	id?: string;
	title: string;
	startISO: string;
	endISO: string;
	isAllDay?: boolean;
	note?: string;
};

export async function signInToCalendar(page: Page): Promise<void> {
	await page.goto('/example-co/calendar');
	const email = page.getByRole('textbox', { name: '이메일' });
	const needsSignIn = await email
		.waitFor({ state: 'visible', timeout: 8000 })
		.then(() => true)
		.catch(() => false);
	if (!needsSignIn) return;
	await email.fill('member1@example.com');
	await page.getByRole('textbox', { name: '비밀번호' }).fill('seed-password');
	await page.getByRole('button', { name: '로그인', exact: true }).click();
	await page.waitForURL('**/example-co/calendar**');
	await page.locator('iframe').waitFor({ state: 'visible' });
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
	return inserted.data.map((row) => row.id as string);
}

export async function cleanupCalendarEvents(eventIDs: string[]): Promise<void> {
	if (eventIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('task').delete().in('id', eventIDs);
	if (deleted.error) throw new Error(`Failed to clean up calendar events: ${deleted.error.message}`);
}

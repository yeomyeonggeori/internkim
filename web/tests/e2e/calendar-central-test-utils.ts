import { createClient } from '@supabase/supabase-js';
import type { Page } from '@playwright/test';

export type CalendarCentralEvent = {
	id?: string;
	title: string;
	startISO: string;
	endISO: string;
	isAllDay?: boolean;
	note?: string;
};

const exampleCompanyID = '000000cc-0000-0000-0000-000000000001';
export const member1ID = '000000ee-0000-0000-0000-000000000001';
export const member2ID = '000000ee-0000-0000-0000-000000000002';
export const member3ID = '000000ee-0000-0000-0000-000000000003';

function centralPlaneAdminClient() {
	const projectURL = process.env.SUPABASE_URL;
	const secretKey = process.env.SUPABASE_SECRET_KEY;
	if (!projectURL || !secretKey) {
		throw new Error('SUPABASE_URL and SUPABASE_SECRET_KEY must be set to seed calendar fixtures');
	}
	return createClient(projectURL, secretKey);
}

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

export type CalendarCentralLeave = {
	memberID: string;
	kind: string;
	days: number;
	startISO: string;
	endISO: string;
};

export async function seedApprovedLeave(leave: CalendarCentralLeave[]): Promise<string[]> {
	const admin = centralPlaneAdminClient();
	const rows = leave.map((entry) => ({
		member_id: entry.memberID,
		kind: entry.kind,
		days: entry.days,
		is_paid: true,
		status: 'approved' as const,
		starts_at: entry.startISO,
		ends_at: entry.endISO
	}));
	const inserted = await admin.from('leave').insert(rows).select('id');
	if (inserted.error) throw new Error(`Failed to seed approved leave: ${inserted.error.message}`);
	return inserted.data.map((row) => row.id as string);
}

export async function cleanupApprovedLeave(leaveIDs: string[]): Promise<void> {
	if (leaveIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('leave').delete().in('id', leaveIDs);
	if (deleted.error) throw new Error(`Failed to clean up approved leave: ${deleted.error.message}`);
}

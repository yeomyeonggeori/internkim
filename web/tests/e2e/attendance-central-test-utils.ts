import type { Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { centralPlaneAdminClient, exampleCompanyID } from './central-test-utils';

export type AttendanceCentralKind = 'clock_in' | 'clock_out';

export type AttendanceCentralEvent = {
	memberID: string;
	kind: AttendanceCentralKind;
	occurredAtISO: string;
	location?: string;
	originalOccurredAtISO?: string;
	editReason?: string;
};

export type AttendanceCentralRow = {
	id: string;
	member_id: string;
	kind: AttendanceCentralKind;
	location: string | null;
	occurred_at: string;
};

export type LeaveCentralRow = {
	id: string;
	member_id: string;
	kind: string;
	days: number;
	status: string;
	starts_at: string;
	ends_at: string;
	note: string | null;
};

const seoulOffset = '+09:00';

export async function signInToAttendance(page: Page, path = '/example-co/attendance'): Promise<void> {
	await signInToTheCentralPlane(page, path);
	await page
		.locator('[data-testid="personal-tools-panel"]:visible')
		.first()
		.waitFor({ state: 'visible', timeout: 30000 });
}

export function seoulDateToday(): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(new Date());
}

export function seoulMonthToday(): string {
	return seoulDateToday().slice(0, 7);
}

export function monthBefore(month: string): string {
	const [year, monthNumber] = month.split('-').map(Number);
	const moved = new Date(Date.UTC(year, monthNumber - 2, 1));
	return `${moved.getUTCFullYear()}-${String(moved.getUTCMonth() + 1).padStart(2, '0')}`;
}

export function dayOfMonth(month: string, day: number): string {
	return `${month}-${String(day).padStart(2, '0')}`;
}

export function seoulInstant(date: string, time: string): string {
	return `${date}T${time}:00${seoulOffset}`;
}

export async function seedAttendanceEvents(events: AttendanceCentralEvent[]): Promise<string[]> {
	const admin = centralPlaneAdminClient();
	const identifiers: string[] = [];
	for (const event of events) {
		const inserted = await admin
			.from('attendance')
			.insert({
				member_id: event.memberID,
				kind: event.kind,
				location: event.kind === 'clock_in' ? (event.location ?? null) : null,
				occurred_at: event.occurredAtISO,
				original_occurred_at: event.originalOccurredAtISO ?? null,
				edit_reason: event.editReason ?? null
			})
			.select('id')
			.single<{ id: string }>();
		if (inserted.error) {
			await cleanupAttendanceEvents(identifiers);
			throw new Error(`Failed to seed an attendance event: ${inserted.error.message}`);
		}
		identifiers.push(inserted.data.id);
	}
	return identifiers;
}

export async function cleanupAttendanceEvents(eventIDs: string[]): Promise<void> {
	if (eventIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('attendance').delete().in('id', eventIDs);
	if (deleted.error) {
		throw new Error(`Failed to clean up attendance events: ${deleted.error.message}`);
	}
}

export async function attendanceRowsOf(memberID: string): Promise<AttendanceCentralRow[]> {
	const admin = centralPlaneAdminClient();
	const rows = await admin
		.from('attendance')
		.select('id, member_id, kind, location, occurred_at')
		.eq('member_id', memberID)
		.order('occurred_at', { ascending: true })
		.returns<AttendanceCentralRow[]>();
	if (rows.error) throw new Error(`Failed to read attendance: ${rows.error.message}`);
	return rows.data;
}

export async function removeAttendanceOf(memberID: string): Promise<void> {
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('attendance').delete().eq('member_id', memberID);
	if (deleted.error) throw new Error(`Failed to clear attendance: ${deleted.error.message}`);
}

export async function leaveRowsOf(memberID: string): Promise<LeaveCentralRow[]> {
	const admin = centralPlaneAdminClient();
	const rows = await admin
		.from('leave')
		.select('id, member_id, kind, days, status, starts_at, ends_at, note')
		.eq('member_id', memberID)
		.lt('days', 0)
		.order('starts_at', { ascending: true })
		.returns<LeaveCentralRow[]>();
	if (rows.error) throw new Error(`Failed to read leave: ${rows.error.message}`);
	return rows.data.map((row) => ({ ...row, days: -row.days }));
}

export async function renameMember(memberID: string, name: string): Promise<void> {
	const admin = centralPlaneAdminClient();
	const updated = await admin.from('member').update({ name }).eq('id', memberID);
	if (updated.error) throw new Error(`Failed to rename a member: ${updated.error.message}`);
}

export async function removeCompanyWorkPolicy(): Promise<void> {
	const admin = centralPlaneAdminClient();
	const company = await admin
		.from('company')
		.select('rules')
		.eq('id', exampleCompanyID)
		.single<{ rules: Record<string, unknown> }>();
	if (company.error) throw new Error(`Failed to read the company rules: ${company.error.message}`);
	const { attendanceWorkPolicy: _attendanceWorkPolicy, ...remainingRules } = company.data.rules;
	const updated = await admin
		.from('company')
		.update({ rules: remainingRules })
		.eq('id', exampleCompanyID);
	if (updated.error) throw new Error(`Failed to remove the work policy: ${updated.error.message}`);
}

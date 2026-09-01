import type { SupabaseClient } from '@supabase/supabase-js';
import type { Environment } from './agent-request';
import { tell, type Told } from './tell';

type AttendanceRow = {
	id: string;
	member_id: string;
	kind: string;
	location: string | null;
	occurred_at: string;
	edit_reason: string | null;
};

type Writer = { name: string | null; company_id: string; timezone: string };

export type Announced = { told: number; messaged: number; failures: string[] };

const attendancePath = '/attendance/';

const wrote: Record<string, string> = {
	added: '추가',
	corrected: '수정',
	removed: '삭제'
};

export async function announceHandWrittenAttendance(
	environment: Environment,
	record: SupabaseClient,
	writerID: string,
	eventID: string,
	status: string
): Promise<Announced> {
	const written = await attendanceOf(record, eventID);
	if (!written) return { told: 0, messaged: 0, failures: [] };

	const writer = await writerOf(record, written.member_id);
	const administrators = await administratorsOf(record, writer.company_id, writerID);

	const deliveries = await Promise.all(
		administrators.map((memberID) =>
			tell(environment, {
				memberID,
				category: 'attendance',
				title: `지난 근태 기록: ${nameOf(writer)}`,
				body: bodyOf(written, writer, status),
				openPath: attendancePath
			})
		)
	);
	return announcedOf(deliveries);
}

function announcedOf(deliveries: Told[]): Announced {
	return {
		told: deliveries.filter((delivery) => delivery.pushed).length,
		messaged: deliveries.filter((delivery) => delivery.messaged).length,
		failures: deliveries.flatMap((delivery) => (delivery.failure ? [delivery.failure] : []))
	};
}

async function attendanceOf(record: SupabaseClient, eventID: string): Promise<AttendanceRow | null> {
	const { data, error } = await record
		.from('attendance')
		.select('id, member_id, kind, location, occurred_at, edit_reason')
		.eq('id', eventID)
		.maybeSingle<AttendanceRow>();
	if (error) throw new Error(error.message);
	return data;
}

async function writerOf(record: SupabaseClient, memberID: string): Promise<Writer> {
	const { data, error } = await record
		.from('member')
		.select('name, company_id, company(timezone)')
		.eq('id', memberID)
		.single<{ name: string | null; company_id: string; company: { timezone: string } | null }>();
	if (error) throw new Error(error.message);
	return { name: data.name, company_id: data.company_id, timezone: data.company?.timezone ?? 'UTC' };
}

async function administratorsOf(
	record: SupabaseClient,
	companyID: string,
	writerID: string
): Promise<string[]> {
	const { data, error } = await record
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq('is_admin', true)
		.neq('status', 'withdrawn')
		.returns<{ id: string }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => member.id).filter((memberID) => memberID !== writerID);
}

function nameOf(writer: Writer): string {
	return (writer.name ?? '').trim() || '누군가';
}

function bodyOf(written: AttendanceRow, writer: Writer, status: string): string {
	const kind = written.kind === 'clock_in' ? '출근' : '퇴근';
	const moment = momentOf(writer.timezone, written.occurred_at);
	const reason = (written.edit_reason ?? '').trim();
	return [`${kind} ${moment} ${wrote[status] ?? status}`, reason === '' ? '' : `사유: ${reason}`]
		.filter((part) => part !== '')
		.join(' · ');
}

function momentOf(timezone: string, occurredAt: string): string {
	return new Intl.DateTimeFormat('sv-SE', {
		timeZone: timezone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).format(new Date(occurredAt));
}

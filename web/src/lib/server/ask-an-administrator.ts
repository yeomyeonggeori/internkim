import type { SupabaseClient } from '@supabase/supabase-js';
import type { Environment } from './agent-request';
import { tell, type Told } from './tell';

export type AttendanceAsked = {
	kind?: string;
	date?: string;
	time?: string;
	location?: string;
	reason?: string;
	eventHint?: string;
};

export type Asked = { told: number; messaged: number; failures: string[] };

type Asker = { name: string | null; company_id: string };

const attendancePath = '/attendance/';

const wanted: Record<string, string> = {
	attendance_add: '지난 근태 기록 추가',
	attendance_update: '지난 근태 기록 수정',
	attendance_delete: '지난 근태 기록 삭제'
};

export async function askAnAdministrator(
	environment: Environment,
	record: SupabaseClient,
	askerID: string,
	toolName: string,
	asked: AttendanceAsked
): Promise<Asked> {
	const asker = await askerOf(record, askerID);
	const administrators = await administratorsOf(record, asker.company_id, askerID);

	const deliveries = await Promise.all(
		administrators.map((memberID) =>
			tell(environment, {
				memberID,
				category: 'attendance',
				title: `${wanted[toolName] ?? toolName}: ${nameOf(asker)}`,
				body: bodyOf(asked),
				openPath: attendancePath
			})
		)
	);
	return askedOf(deliveries);
}

function askedOf(deliveries: Told[]): Asked {
	return {
		told: deliveries.filter((delivery) => delivery.pushed).length,
		messaged: deliveries.filter((delivery) => delivery.messaged).length,
		failures: deliveries.flatMap((delivery) => (delivery.failure ? [delivery.failure] : []))
	};
}

async function askerOf(record: SupabaseClient, memberID: string): Promise<Asker> {
	const { data, error } = await record
		.from('member')
		.select('name, company_id')
		.eq('id', memberID)
		.single<Asker>();
	if (error) throw new Error(error.message);
	return data;
}

async function administratorsOf(
	record: SupabaseClient,
	companyID: string,
	askerID: string
): Promise<string[]> {
	const { data, error } = await record
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq('is_admin', true)
		.neq('status', 'withdrawn')
		.returns<{ id: string }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => member.id).filter((memberID) => memberID !== askerID);
}

function nameOf(asker: Asker): string {
	return (asker.name ?? '').trim() || '누군가';
}

function bodyOf(asked: AttendanceAsked): string {
	const kind = asked.kind === 'clock_in' ? '출근' : asked.kind === 'clock_out' ? '퇴근' : '';
	const moment = [asked.date, asked.time].filter((part) => (part ?? '').trim() !== '').join(' ');
	const reason = (asked.reason ?? '').trim();
	return [
		[kind, moment].filter((part) => part !== '').join(' '),
		(asked.eventHint ?? '').trim(),
		reason === '' ? '' : `사유: ${reason}`
	]
		.filter((part) => part !== '')
		.join(' · ');
}

import type { SupabaseClient } from '@supabase/supabase-js';
import type { Environment } from './agent-request';
import { tell, type Told } from './tell';

type ApprovalRow = {
	id: string;
	member_id: string;
	kind: string;
	payload: Record<string, unknown>;
	reason: string;
	status: string;
};

type Asker = { name: string | null; company_id: string };

export type Announced = { told: number; messaged: number; failures: string[] };

const asks: Record<string, string> = {
	attendance_add: '근태 기록 추가',
	attendance_edit: '근태 기록 수정',
	attendance_remove: '근태 기록 삭제'
};

export async function announceApproval(
	environment: Environment,
	record: SupabaseClient,
	askerID: string,
	approvalID: string
): Promise<Announced> {
	const asked = await approvalOf(record, approvalID);
	if (!asked || asked.status !== 'pending') return { told: 0, messaged: 0, failures: [] };
	if (asked.member_id !== askerID) return { told: 0, messaged: 0, failures: [] };

	const asker = await askerOf(record, asked.member_id);
	const administrators = await administratorsOf(record, asker.company_id, asked.member_id);

	const deliveries = await Promise.all(
		administrators.map((memberID) =>
			tell(environment, {
				memberID,
				category: 'approval',
				title: `결재 요청: ${nameOf(asker)}`,
				body: bodyOf(asked)
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

async function approvalOf(record: SupabaseClient, approvalID: string): Promise<ApprovalRow | null> {
	const { data, error } = await record
		.from('approval')
		.select('id, member_id, kind, payload, reason, status')
		.eq('id', approvalID)
		.maybeSingle<ApprovalRow>();
	if (error) throw new Error(error.message);
	return data;
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

function bodyOf(asked: ApprovalRow): string {
	return [asks[asked.kind] ?? asked.kind, whatFor(asked), `사유: ${asked.reason}`]
		.filter((part) => part !== '')
		.join(' · ');
}

function whatFor(asked: ApprovalRow): string {
	if (asked.kind !== 'attendance_add') return '';
	const kind = asked.payload.kind === 'clock_in' ? '출근' : '퇴근';
	return [kind, asked.payload.localDate, asked.payload.localTime]
		.filter((part) => typeof part === 'string' && part !== '')
		.join(' ');
}

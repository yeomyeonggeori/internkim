import { statusOfPostgresCode, RecordRefusedTheWrite } from './tasks';
import type { RecordContext } from './company';

export type PendingApprovalRow = {
	id: string;
	member_id: string;
	asked_by: string;
	kind: string;
	payload: Record<string, unknown>;
	reason: string;
	created_at: string;
};

export class NoSuchApproval extends Error {
	constructor(
		readonly hint: string,
		readonly candidates: string[]
	) {
		super(
			candidates.length === 0
				? `no request here goes by ${hint}`
				: `${hint} could be ${candidates.join(', ')}; name one of them exactly`
		);
		this.name = 'NoSuchApproval';
	}
}

export type AnsweredApproval = {
	approvalID: string;
	askedBy: string;
	kind: string;
	asks: string;
	reason: string;
	askedAt: string;
};

const asked: Record<string, string> = {
	attendance_add: 'add an attendance record',
	attendance_edit: 'correct an attendance record',
	attendance_remove: 'remove an attendance record'
};

function whatItAsks(row: PendingApprovalRow): string {
	const verb = asked[row.kind] ?? row.kind;
	if (row.kind === 'attendance_add') {
		return `${verb}: ${row.payload.kind} ${row.payload.localDate} ${row.payload.localTime}`;
	}
	return verb;
}

function answeredApproval(row: PendingApprovalRow): AnsweredApproval {
	return {
		approvalID: row.id,
		askedBy: row.asked_by,
		kind: row.kind,
		asks: whatItAsks(row),
		reason: row.reason,
		askedAt: row.created_at
	};
}

function describedApproval(row: PendingApprovalRow): string {
	const answered = answeredApproval(row);
	return `${answered.askedBy} · ${answered.asks}`;
}

async function pendingApprovals(context: RecordContext): Promise<PendingApprovalRow[]> {
	const { data, error } = await context.caller.rpc('approval_pending');
	if (error) throw new Error(error.message);
	return (data ?? []) as PendingApprovalRow[];
}

function approvalOfHint(rows: PendingApprovalRow[], hint: string): PendingApprovalRow {
	const asking = hint.trim();
	if (!asking) throw new NoSuchApproval(hint, []);

	const byID = rows.find((row) => row.id === asking);
	if (byID) return byID;

	const described = rows.filter((row) => describedApproval(row).includes(asking));
	if (described.length === 1) return described[0];
	throw new NoSuchApproval(hint, described.map(describedApproval));
}

export async function approvalList(context: RecordContext) {
	const rows = await pendingApprovals(context);
	return { count: rows.length, approvals: rows.map(answeredApproval) };
}

export type ApprovalDecideInput = { approvalHint?: string; decision?: string; note?: string };

export async function approvalDecide(context: RecordContext, input: ApprovalDecideInput) {
	if (!input.approvalHint) throw new Error('a decision names the request it decides');
	if (input.decision !== 'approved' && input.decision !== 'rejected') {
		throw new Error('a decision is approved or rejected');
	}

	const row = approvalOfHint(await pendingApprovals(context), input.approvalHint);
	const { data, error } = await context.caller.rpc('approval_decide', {
		approval_id: row.id,
		decision: input.decision,
		note: input.note?.trim() || null
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	const settled = (data ?? {}) as { status?: string; applied?: unknown };
	return {
		approvalID: row.id,
		askedBy: row.asked_by,
		asks: whatItAsks(row),
		status: settled.status ?? input.decision,
		applied: settled.applied ?? null
	};
}

export type ReplacingLeave = { id: string; original_leave_id: string | null };

export async function leaveReplacedByPending<Leave extends ReplacingLeave>(
	pending: Leave[],
	readApproved: () => Promise<Leave[]>
): Promise<Map<string, Leave>> {
	const replaced = new Set(pending.flatMap((leave) => (leave.original_leave_id ? [leave.original_leave_id] : [])));
	if (replaced.size === 0) return new Map();
	const approved = await readApproved();
	return new Map(approved.filter((leave) => replaced.has(leave.id)).map((leave) => [leave.id, leave]));
}

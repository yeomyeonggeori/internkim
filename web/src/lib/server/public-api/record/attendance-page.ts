import type { SupabaseClient } from '@supabase/supabase-js';
import { heldColumns, type AttendanceRow } from './attendance';
import { fetchedPageSize, pageOfFetched, positionOfCursor, type ListPage, type ListPageInput } from './list-page';
import { RecordReadFailed } from './tasks';
import type { WhoseRecords } from './whose';

export type AttendancePageScope = {
	whose: WhoseRecords;
	from: string;
	to: string;
	handWrittenOnly: boolean;
};

const resourceIDPattern = /^[\w-]+$/;

export async function attendancePage(caller: SupabaseClient, scope: AttendancePageScope, input: ListPageInput): Promise<ListPage<AttendanceRow>> {
	const before = input.cursor === undefined ? undefined : positionOfCursor('attendance_list', input.cursor, isAttendancePosition);
	if (!scope.whose.everyone && scope.whose.personIDs.length === 0) return { items: [], totalCount: 0 };
	const listed = scopedAttendance(caller, scope, heldColumns);
	const [page, total] = await Promise.all([
		(before ? listed.or(beforeClause(before)) : listed)
			.order('occurred_at', { ascending: false })
			.order('id', { ascending: false })
			.limit(fetchedPageSize(input))
			.returns<AttendanceRow[]>(),
		scopedAttendance(caller, scope, 'id', { count: 'exact' }).limit(1)
	]);
	for (const answer of [page, total]) if (answer.error) throw new RecordReadFailed(answer);
	const newestFirst = pageOfFetched(page.data ?? [], total.count ?? 0, (row) => [row.occurred_at, row.id], input);
	return { ...newestFirst, items: newestFirst.items.toReversed() };
}

function scopedAttendance(caller: SupabaseClient, scope: AttendancePageScope, columns: string, options?: { count: 'exact' }) {
	const inWindow = caller.from('attendance').select(columns, options).gte('occurred_at', scope.from).lte('occurred_at', scope.to);
	const written = scope.handWrittenOnly ? inWindow.or('edit_reason.not.is.null,original_occurred_at.not.is.null') : inWindow;
	return scope.whose.everyone ? written : written.in('member_id', scope.whose.personIDs);
}

function beforeClause([occurredAt, eventID]: string[]): string {
	const instant = JSON.stringify(occurredAt);
	return `or(occurred_at.lt.${instant},and(occurred_at.eq.${instant},id.lt.${eventID}))`;
}

function isAttendancePosition(position: string[]): boolean {
	const [occurredAt, eventID] = position;
	return position.length === 2 && !Number.isNaN(Date.parse(occurredAt)) && resourceIDPattern.test(eventID);
}

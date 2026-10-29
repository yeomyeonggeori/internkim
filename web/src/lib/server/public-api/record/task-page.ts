import type { SupabaseClient } from '@supabase/supabase-js';
import { isTaskStatusFinished } from '../../../../routes/task/task-status';
import { WorkspaceTaskStatus } from '../catalog/workspace-task';
import { fetchedPageSize, pageOfFetched, positionOfCursor, type ListPage, type ListPageInput } from './list-page';
import { cardColumns, type TaskCardRow } from './task-board';
import { workColumn } from './tasks';
import type { WhoseRecords } from './whose';

export type TaskSearch = { text: string; soleParticipantIDs: string[] };

export type TaskPageScope = {
	whose: WhoseRecords;
	status?: string;
	organizationID?: string;
	opportunityID?: string;
	dates?: string;
	search?: TaskSearch;
	carriesLinks: boolean;
};

export type TaskPage = ListPage<TaskCardRow> & { unfinishedCount: number };

const finishedStatuses = Object.values(WorkspaceTaskStatus).filter(isTaskStatusFinished);
const resourceIDPattern = /^[\w-]+$/;

export async function taskPage(caller: SupabaseClient, scope: TaskPageScope, input: ListPageInput, toolName: string): Promise<TaskPage> {
	const after = input.cursor === undefined ? undefined : positionOfCursor(toolName, input.cursor, isTaskPosition);
	if (!scope.whose.everyone && scope.whose.personIDs.length === 0) return { items: [], totalCount: 0, unfinishedCount: 0 };
	const listed = scopedTasks(caller, scope, selectionOf(scope));
	const [page, total, unfinished] = await Promise.all([
		(after ? listed.or(afterClause(after)) : listed)
			.order('updated_at', { ascending: false })
			.order('id')
			.limit(fetchedPageSize(input))
			.returns<TaskCardRow[]>(),
		scopedTasks(caller, scope, countSelectionOf(scope), { count: 'exact', head: true }),
		scopedTasks(caller, scope, countSelectionOf(scope), { count: 'exact', head: true })
			.not('status', 'in', `(${finishedStatuses.join(',')})`)
	]);
	for (const answer of [page, total, unfinished]) if (answer.error) throw new Error(answer.error.message);
	return {
		...pageOfFetched(page.data ?? [], total.count ?? 0, (row) => [row.updated_at, row.id], input),
		unfinishedCount: unfinished.count ?? 0
	};
}

function scopedTasks(caller: SupabaseClient, scope: TaskPageScope, columns: string, options?: { count: 'exact'; head: true }) {
	const scoped = caller.from('task').select(columns, options).or(`and(${clausesOf(scope).join(',')})`);
	return scope.whose.everyone ? scoped : scoped.in('participant.member_id', scope.whose.personIDs);
}

function clausesOf(scope: TaskPageScope): string[] {
	return [
		`${workColumn}.eq.true`,
		...(scope.status ? [`status.eq.${scope.status}`] : []),
		...(scope.organizationID ? [`organization_id.eq.${scope.organizationID}`] : []),
		...(scope.opportunityID ? [`opportunity_id.eq.${scope.opportunityID}`] : []),
		...(scope.dates ? [`or(${scope.dates})`] : []),
		...(scope.search ? [searchClause(scope.search)] : [])
	];
}

function searchClause(search: TaskSearch): string {
	const pattern = JSON.stringify(`%${search.text.replace(/[\\%_]/g, '\\$&')}%`);
	const owners = search.soleParticipantIDs.length > 0 ? [`task_sole_participant.in.(${search.soleParticipantIDs.join(',')})`] : [];
	return `or(${[`task_search_text.like.${pattern}`, ...owners].join(',')})`;
}

function afterClause([updatedAt, taskID]: string[]): string {
	const instant = JSON.stringify(updatedAt);
	return `or(updated_at.lt.${instant},and(updated_at.eq.${instant},id.gt.${taskID}))`;
}

function isTaskPosition(position: string[]): boolean {
	const [updatedAt, taskID] = position;
	return position.length === 2 && !Number.isNaN(Date.parse(updatedAt)) && resourceIDPattern.test(taskID);
}

function selectionOf(scope: TaskPageScope): string {
	return [cardColumns, ...(scope.carriesLinks ? ['organization_id', 'opportunity_id'] : []), ...participantFilterOf(scope)].join(', ');
}

function countSelectionOf(scope: TaskPageScope): string {
	return ['id', ...participantFilterOf(scope)].join(', ');
}

function participantFilterOf(scope: TaskPageScope): string[] {
	return scope.whose.everyone ? [] : ['participant:task_participant!inner(member_id)'];
}

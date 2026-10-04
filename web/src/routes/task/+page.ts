import { browser } from '$app/environment';
import { projectURL } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';
import { fetchTaskState, taskStateDependency, mergeTaskSummary, taskWeeklySummaryOf } from './task-api';
import { taskAccountScope } from '$lib/task/task-account-scope';
import { taskBoardState } from '$lib/task/task-state';
import { ToolRefused } from '$lib/public-api-call';
import { lastSeenTask } from './task-last-seen';
import { taskSnapshotGeneration } from './task-snapshot-storage';
import { taskWeekForCode } from '$lib/task/task-week-code';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent, depends, url }) => {
	depends(taskStateDependency);
	const { session } = await parent();
	if (!browser || !session?.authenticated) return { taskRead: null, taskBoardRead: null, taskScope: '', lastTask: null };
	const member = await supabaseMember();
	if (!member.companyID || !member.memberID) return { taskRead: null, taskBoardRead: null, taskScope: '', lastTask: null };
	const taskScope = taskAccountScope(member, projectURL());
	const taskReadGeneration = taskSnapshotGeneration();
	const taskWeek = taskWeekForCode(url.searchParams.get('week') ?? '', new Date());
	const cached = lastSeenTask(taskScope, taskWeek.startISO);
	const lastTask = cached.state ? { ...cached, summary: mergeTaskSummary(cached.state, taskWeeklySummaryOf(cached.state, url.searchParams.get('week') ?? '')) } : cached;
	const taskRead = fetchTaskState(taskScope, taskWeek.startISO).then(
		(state) => ({ state, error: '', denied: false }),
		(error: unknown) => ({ state: null, error: error instanceof Error ? error.message : String(error), denied: error instanceof ToolRefused && (error.status === 401 || error.status === 403) })
	);
	const taskViewer = { memberID: member.memberID, email: session.email, name: member.name, isAdmin: member.role === 'admin' };
	const taskBoardRead = taskBoardState(taskScope, taskViewer, taskWeek.startISO).catch(() => null);
	return { taskRead, taskBoardRead, taskScope, taskReadGeneration, taskWeek, taskViewer, lastTask };
};

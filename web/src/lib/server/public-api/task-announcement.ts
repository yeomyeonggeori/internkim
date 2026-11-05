import type { Environment } from '$lib/server/agent-request';
import { askTheProject } from '$lib/server/project-function';
import { changesAnythingAnnounced, type TaskChange, type TaskChangeAnnouncer } from './record/task-change';

type BackgroundWork = (work: Promise<unknown>) => void;

export function taskChangesAnnouncedAs(
	environment: Environment,
	accessToken: string,
	backgroundWork?: BackgroundWork
): TaskChangeAnnouncer {
	return async (change) => {
		if (!changesAnythingAnnounced(change)) return;
		const announcement = announce(environment, accessToken, change);
		if (backgroundWork) backgroundWork(announcement);
		else await announcement;
	};
}

async function announce(environment: Environment, accessToken: string, change: TaskChange): Promise<void> {
	try {
		const answer = await askTheProject(environment, 'announce-task', change, accessToken);
		if (answer.status >= 300) {
			console.error('task.announcement_failed', { taskID: change.taskID, status: answer.status, body: answer.body });
		}
	} catch (failure) {
		console.error('task.announcement_failed', { taskID: change.taskID, failure });
	}
}

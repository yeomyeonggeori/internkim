import { isTaskStatusCompleted } from '../../task/task-status';
import type { TaskState, Task } from '../../task/task-types';
import type { TeamStatusCompletedTaskDetail, TeamStatusDayContextPerson } from './team-status-day-context';

export function completedTasksForPersonDay(
	person: TeamStatusDayContextPerson,
	date: string,
	taskState: TaskState | null
): TeamStatusCompletedTaskDetail[] {
	if (!taskState) return [];
	return taskState.tasks
		.filter((task) => isCompletedTaskForPersonDay(task, person, date, taskState))
		.map(completedTaskDetail);
}

function isCompletedTaskForPersonDay(
	task: Task,
	person: TeamStatusDayContextPerson,
	date: string,
	taskState: TaskState
): boolean {
	if (!isTaskStatusCompleted(task.status)) return false;
	if ((task.endDate ?? '').trim() !== date) return false;
	return taskMatchesPerson(task, person, taskState);
}

function taskMatchesPerson(task: Task, person: TeamStatusDayContextPerson, taskState: TaskState): boolean {
	const emailMemberIDs = taskState.members
		.filter((member) => normalizeToken(member.email) === normalizeToken(person.email))
		.map((member) => member.id);
	if (emailMemberIDs.length > 0) return taskMatchesMemberIDs(task, emailMemberIDs);
	const memberIDs = taskState.members
		.filter((member) => normalizeToken(member.name) === normalizeToken(person.displayName))
		.map((member) => member.id);
	const personTokens = personMatchTokens(person);
	if (taskMatchesMemberIDs(task, memberIDs)) return true;
	if (personTokens.has(normalizeToken(task.ownerName))) return true;
	return task.participantNames.some((name) => personTokens.has(normalizeToken(name)));
}

function taskMatchesMemberIDs(task: Task, memberIDs: string[]): boolean {
	if (memberIDs.includes(task.ownerID)) return true;
	return task.participantIDs.some((participantID) => memberIDs.includes(participantID));
}

function personMatchTokens(person: TeamStatusDayContextPerson): Set<string> {
	return new Set([person.email, person.displayName].map(normalizeToken).filter(Boolean));
}

function normalizeToken(value: string): string {
	return value.trim().toLowerCase();
}

function completedTaskDetail(task: Task): TeamStatusCompletedTaskDetail {
	return {
		id: task.id,
		title: task.content || '-',
		ownerName: task.ownerName,
		collaboratorNames: collaboratorNames(task),
		task
	};
}

function collaboratorNames(task: Task): string[] {
	const ownerName = normalizeToken(task.ownerName);
	const seenNames = new Set<string>();
	return task.participantNames.filter((name) => {
		const normalizedName = normalizeToken(name);
		if (!normalizedName || normalizedName === ownerName || seenNames.has(normalizedName)) return false;
		seenNames.add(normalizedName);
		return true;
	});
}

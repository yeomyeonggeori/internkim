import { isFlowStatusCompleted } from '../../flow/flow-status';
import type { FlowState, FlowTask } from '../../flow/flow-types';
import type { TeamStatusCompletedTaskDetail, TeamStatusDayContextPerson } from './team-status-day-context';

export function completedFlowTasksForPersonDay(
	person: TeamStatusDayContextPerson,
	date: string,
	flowState: FlowState | null
): TeamStatusCompletedTaskDetail[] {
	if (!flowState) return [];
	return flowState.tasks
		.filter((task) => isCompletedFlowTaskForPersonDay(task, person, date, flowState))
		.map(completedTaskDetail);
}

function isCompletedFlowTaskForPersonDay(
	task: FlowTask,
	person: TeamStatusDayContextPerson,
	date: string,
	flowState: FlowState
): boolean {
	if (!isFlowStatusCompleted(task.status)) return false;
	if ((task.endDate ?? '').trim() !== date) return false;
	return flowTaskMatchesPerson(task, person, flowState);
}

function flowTaskMatchesPerson(task: FlowTask, person: TeamStatusDayContextPerson, flowState: FlowState): boolean {
	const emailMemberIDs = flowState.members
		.filter((member) => normalizeToken(member.email) === normalizeToken(person.email))
		.map((member) => member.id);
	if (emailMemberIDs.length > 0) return flowTaskMatchesMemberIDs(task, emailMemberIDs);
	const memberIDs = flowState.members
		.filter((member) => normalizeToken(member.name) === normalizeToken(person.displayName))
		.map((member) => member.id);
	const personTokens = personMatchTokens(person);
	if (flowTaskMatchesMemberIDs(task, memberIDs)) return true;
	if (personTokens.has(normalizeToken(task.ownerName))) return true;
	return task.participantNames.some((name) => personTokens.has(normalizeToken(name)));
}

function flowTaskMatchesMemberIDs(task: FlowTask, memberIDs: string[]): boolean {
	if (memberIDs.includes(task.ownerID)) return true;
	return task.participantIDs.some((participantID) => memberIDs.includes(participantID));
}

function personMatchTokens(person: TeamStatusDayContextPerson): Set<string> {
	return new Set([person.email, person.displayName, person.mattermostUsername ?? ''].map(normalizeToken).filter(Boolean));
}

function normalizeToken(value: string): string {
	return value.trim().toLowerCase();
}

function completedTaskDetail(task: FlowTask): TeamStatusCompletedTaskDetail {
	return {
		id: task.id,
		title: task.content || task.goal || '-',
		ownerName: task.ownerName,
		collaboratorNames: collaboratorNames(task)
	};
}

function collaboratorNames(task: FlowTask): string[] {
	const ownerName = normalizeToken(task.ownerName);
	const seenNames = new Set<string>();
	return task.participantNames.filter((name) => {
		const normalizedName = normalizeToken(name);
		if (!normalizedName || normalizedName === ownerName || seenNames.has(normalizedName)) return false;
		seenNames.add(normalizedName);
		return true;
	});
}

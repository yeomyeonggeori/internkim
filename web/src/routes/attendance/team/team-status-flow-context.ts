// 출결 팀 상세 팝업의 개인별 완료 Flow 업무 매칭을 담당한다.
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
	const memberIDs = flowState.members
		.filter((member) => normalizeToken(member.email) === normalizeToken(person.email) || normalizeToken(member.name) === normalizeToken(person.displayName))
		.map((member) => member.id);
	const personTokens = personMatchTokens(person);
	if (memberIDs.includes(task.ownerID)) return true;
	if (personTokens.has(normalizeToken(task.ownerName))) return true;
	if (task.participantIDs.some((participantID) => memberIDs.includes(participantID))) return true;
	return task.participantNames.some((name) => personTokens.has(normalizeToken(name)));
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

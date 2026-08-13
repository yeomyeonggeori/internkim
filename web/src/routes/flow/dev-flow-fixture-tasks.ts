import { addDays, weekOffsetFromBaseline } from './dev-flow-fixture-date';
import {
	devFlowBaselineWeekStartISO,
	devFlowFutureTaskSpecs,
	devFlowMembers,
	devFlowTaskSpecs,
	type DevFlowTaskSpec
} from './dev-flow-fixture-data';
import { isFlowStatusCompleted, isFlowStatusInProgress, isFlowStatusPaused, isFlowStatusStopped } from './flow-status';
import type { FlowMember, FlowTask, FlowWeek } from './flow-types';

export function createFixtureTasks(week: FlowWeek): FlowTask[] {
	const tasks = devFlowTaskSpecs.map((spec) => taskFromSpec(spec, week));
	const weekOffset = weekOffsetFromBaseline(week, devFlowBaselineWeekStartISO);
	if (weekOffset < 0) return previousFixtureTasks(tasks, Math.abs(weekOffset));
	if (weekOffset > 0) return [...tasks, ...devFlowFutureTaskSpecs.map((spec) => taskFromSpec(spec, week)).slice(0, Math.min(weekOffset, 4))];
	return tasks;
}

function previousFixtureTasks(tasks: FlowTask[], weekOffset: number): FlowTask[] {
	const completedSizes = ['XS', 'S', 'M'];
	const progressSizes = ['S', 'M', 'L'];
	return tasks.map((task, index) => {
		if (isFlowStatusCompleted(task.status)) return { ...task, size: completedSizes[(index + weekOffset) % completedSizes.length] ?? task.size };
		if (isFlowStatusInProgress(task.status)) return { ...task, size: progressSizes[(index + weekOffset) % progressSizes.length] ?? task.size };
		return task;
	});
}

function taskFromSpec(spec: DevFlowTaskSpec, week: FlowWeek): FlowTask {
	const owner = memberByID(spec.ownerID);
	const participants = spec.participantIDs.map(memberByID);

	return {
		id: spec.id,
		parentTaskID: spec.parentTaskID,
		requesterID: spec.requesterID,
		ownerID: spec.ownerID,
		ownerName: owner.name,
		participantIDs: spec.participantIDs,
		participantNames: participants.map((participant) => participant.name),
		business: spec.business,
		type: spec.type,
		content: spec.content,
		goal: spec.goal,
		size: spec.size,
		status: spec.status,
		statusRank: 0,
		startDate: addDays(week.startISO, spec.startOffset),
		endDate: spec.endOffset > 0 ? addDays(week.startISO, spec.endOffset) : '',
		createdAt: `${addDays(week.startISO, spec.startOffset)}T09:00:00Z`,
		weekCode: week.code,
		flag: isFlowStatusStopped(spec.status) || isFlowStatusPaused(spec.status) ? 1 : 0,
		requestReason: spec.requestReason ?? '',
		decisionReason: ''
	};
}

function memberByID(id: string): FlowMember {
	const found = devFlowMembers.find((member) => member.id === id);
	if (!found) throw new Error(`unknown dev flow member: ${id}`);
	return found;
}

import { addDays, weekOffsetFromBaseline } from './dev-task-fixture-date';
import {
	devTaskBaselineWeekStartISO,
	devTaskFutureTaskSpecs,
	devTaskMembers,
	devTaskSpecs,
	type DevTaskSpec
} from './dev-task-fixture-data';
import { isTaskStatusCompleted, isTaskStatusInProgress, isTaskStatusPaused, isTaskStatusStopped } from './task-status';
import type { TaskMember, Task, TaskWeek } from './task-types';

export function createFixtureTasks(week: TaskWeek): Task[] {
	const tasks = devTaskSpecs.map((spec) => taskFromSpec(spec, week));
	const weekOffset = weekOffsetFromBaseline(week, devTaskBaselineWeekStartISO);
	if (weekOffset < 0) return previousFixtureTasks(tasks, Math.abs(weekOffset));
	if (weekOffset > 0) return [...tasks, ...devTaskFutureTaskSpecs.map((spec) => taskFromSpec(spec, week)).slice(0, Math.min(weekOffset, 4))];
	return tasks;
}

function previousFixtureTasks(tasks: Task[], weekOffset: number): Task[] {
	const completedSizes = ['XS', 'S', 'M'];
	const progressSizes = ['S', 'M', 'L'];
	return tasks.map((task, index) => {
		if (isTaskStatusCompleted(task.status)) return { ...task, size: completedSizes[(index + weekOffset) % completedSizes.length] ?? task.size };
		if (isTaskStatusInProgress(task.status)) return { ...task, size: progressSizes[(index + weekOffset) % progressSizes.length] ?? task.size };
		return task;
	});
}

function taskFromSpec(spec: DevTaskSpec, week: TaskWeek): Task {
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
		size: spec.size,
		status: spec.status,
		statusRank: 0,
		startDate: addDays(week.startISO, spec.startOffset),
		endDate: spec.endOffset > 0 ? addDays(week.startISO, spec.endOffset) : '',
		createdAt: `${addDays(week.startISO, spec.startOffset)}T09:00:00Z`,
		weekCode: week.code
	};
}

function memberByID(id: string): TaskMember {
	const found = devTaskMembers.find((member) => member.id === id);
	if (!found) throw new Error(`unknown dev task member: ${id}`);
	return found;
}

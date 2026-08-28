import { completedDistanceForTask } from './report/task-report-distance';
import { totalDevTaskMemberScore } from './dev-task-fixture-score';
import {
	isTaskStatusCompleted,
	isTaskStatusPaused,
	isTaskStatusRejected,
	isTaskStatusRequested,
	isTaskStatusStopped
} from './task-status';
import type { TaskDefinitions, TaskMember, TaskMetrics, Task } from './task-types';

export type DevTaskMetrics = TaskMetrics & {
	totalDistance: number;
	totalScore: number;
	memberDistances: Record<string, number>;
	memberScores: Record<string, number>;
	memberScoreDetails: NonNullable<TaskMetrics['memberScoreDetails']>;
};

export function buildDevTaskMetrics(
	tasks: Task[],
	definitions: TaskDefinitions,
	memberScores: Record<string, number>,
	memberScoreDetails: NonNullable<TaskMetrics['memberScoreDetails']>
): DevTaskMetrics {
	const metrics: DevTaskMetrics = {
		totalTasks: 0,
		completedTasks: 0,
		requestedTasks: 0,
		pausedTasks: 0,
		stoppedTasks: 0,
		totalDistance: 0,
		totalScore: totalDevTaskMemberScore(memberScores),
		statusCounts: {},
		businessCounts: {},
		typeCounts: {},
		memberDistances: {},
		memberScores: { ...memberScores },
		memberScoreDetails: { ...memberScoreDetails }
	};

	for (const task of tasks) {
		metrics.totalTasks += 1;
		increment(metrics.statusCounts, task.status, 1);
		increment(metrics.businessCounts, task.business, 1);
		increment(metrics.typeCounts, task.type, 1);
		if (isTaskStatusCompleted(task.status)) metrics.completedTasks += 1;
		if (isTaskStatusRequested(task.status)) metrics.requestedTasks += 1;
		if (isTaskStatusPaused(task.status)) metrics.pausedTasks += 1;
		if (isTaskStatusStopped(task.status)) metrics.stoppedTasks += 1;

		const distance = completedDistanceForTask(task, definitions);
		metrics.totalDistance += distance;
		for (const participantName of task.participantNames) {
			increment(metrics.memberDistances, participantName, distance);
		}
	}

	return metrics;
}

export function calculateDevTaskMemberDistances(members: TaskMember[], tasks: Task[], definitions: TaskDefinitions): TaskMember[] {
	return members.map((member) => {
		const memberTasks = tasks.filter((task) => task.participantIDs.includes(member.id));
		const distance = memberTasks.reduce((total, task) => total + completedDistanceForTask(task, definitions), 0);
		return {
			...member,
			distance,
			score: 0,
			activeTaskCount: memberTasks.filter((task) => isActiveMemberTaskStatus(task.status)).length,
			completeTaskCount: memberTasks.filter((task) => isTaskStatusCompleted(task.status)).length
		};
	});
}

export function applyDevTaskMemberScores(members: TaskMember[], scores: Record<string, number>): TaskMember[] {
	return members.map((member) => ({ ...member, score: scores[member.id] ?? 0 }));
}

function isActiveMemberTaskStatus(status: string): boolean {
	return !isTaskStatusCompleted(status) && !isTaskStatusRejected(status) && !isTaskStatusStopped(status);
}

function increment(record: Record<string, number>, key: string, amount: number): void {
	record[key] = (record[key] ?? 0) + amount;
}

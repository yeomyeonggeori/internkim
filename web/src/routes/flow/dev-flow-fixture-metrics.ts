import { completedDistanceForTask } from './report/flow-report-distance';
import { totalDevFlowMemberScore } from './dev-flow-fixture-score';
import {
	isFlowStatusCompleted,
	isFlowStatusPaused,
	isFlowStatusRejected,
	isFlowStatusRequested,
	isFlowStatusStopped
} from './flow-status';
import type { FlowDefinitions, FlowMember, FlowMetrics, FlowTask } from './flow-types';

export type DevFlowMetrics = FlowMetrics & {
	totalDistance: number;
	totalScore: number;
	memberDistances: Record<string, number>;
	memberScores: Record<string, number>;
	memberScoreDetails: NonNullable<FlowMetrics['memberScoreDetails']>;
};

export function buildDevFlowMetrics(
	tasks: FlowTask[],
	definitions: FlowDefinitions,
	memberScores: Record<string, number>,
	memberScoreDetails: NonNullable<FlowMetrics['memberScoreDetails']>
): DevFlowMetrics {
	const metrics: DevFlowMetrics = {
		totalTasks: 0,
		completedTasks: 0,
		requestedTasks: 0,
		pausedTasks: 0,
		stoppedTasks: 0,
		totalDistance: 0,
		totalScore: totalDevFlowMemberScore(memberScores),
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
		if (isFlowStatusCompleted(task.status)) metrics.completedTasks += 1;
		if (isFlowStatusRequested(task.status)) metrics.requestedTasks += 1;
		if (isFlowStatusPaused(task.status)) metrics.pausedTasks += 1;
		if (isFlowStatusStopped(task.status)) metrics.stoppedTasks += 1;

		const distance = completedDistanceForTask(task, definitions);
		metrics.totalDistance += distance;
		for (const participantName of task.participantNames) {
			increment(metrics.memberDistances, participantName, distance);
		}
	}

	return metrics;
}

export function calculateDevFlowMemberDistances(members: FlowMember[], tasks: FlowTask[], definitions: FlowDefinitions): FlowMember[] {
	return members.map((member) => {
		const memberTasks = tasks.filter((task) => task.participantIDs.includes(member.id));
		const distance = memberTasks.reduce((total, task) => total + completedDistanceForTask(task, definitions), 0);
		return {
			...member,
			distance,
			score: 0,
			activeTaskCount: memberTasks.filter((task) => isActiveMemberTaskStatus(task.status)).length,
			completeTaskCount: memberTasks.filter((task) => isFlowStatusCompleted(task.status)).length
		};
	});
}

export function applyDevFlowMemberScores(members: FlowMember[], scores: Record<string, number>): FlowMember[] {
	return members.map((member) => ({ ...member, score: scores[member.id] ?? 0 }));
}

function isActiveMemberTaskStatus(status: string): boolean {
	return !isFlowStatusCompleted(status) && !isFlowStatusRejected(status) && !isFlowStatusStopped(status);
}

function increment(record: Record<string, number>, key: string, amount: number): void {
	record[key] = (record[key] ?? 0) + amount;
}

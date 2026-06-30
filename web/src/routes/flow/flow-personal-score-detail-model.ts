import { isFlowStatusCompleted } from './flow-status';
import { currentFlowMember } from './flow-task-workspace-model';
import type { FlowDefinitions, FlowMember, FlowSummary, FlowTask, FlowWeek } from './flow-types';
import { completedDistanceForTask } from './report/flow-report-distance';

const scorePeriodCount = 5;
const scoreWeights = [1.5, 1.4, 1.3, 1.2, 1.1] as const;

export type FlowPersonalScoreDetail = {
	memberName: string;
	weekly: FlowPersonalScorePeriod;
	monthly: FlowPersonalScorePeriod;
};

export type FlowPersonalScorePeriod = {
	rows: FlowPersonalScoreRow[];
	totalScore: number;
};

export type FlowPersonalScoreRow = {
	periodIndex: number;
	completedDistance: number;
	cumulativeAverage: number;
	unitScore: number;
	weight: number;
	weightedScore: number;
};

export function buildFlowPersonalScoreDetail(summary: FlowSummary | null): FlowPersonalScoreDetail | null {
	const member = currentFlowMember(summary);
	if (!summary || !member) return null;
	return {
		memberName: member.name,
		weekly: buildScorePeriod(periodDistances(summary.tasks, summary.members, summary.definitions, summary.week, member, scoreWeekIndex)),
		monthly: buildScorePeriod(periodDistances(summary.tasks, summary.members, summary.definitions, summary.week, member, scoreMonthIndex))
	};
}

function buildScorePeriod(distances: number[]): FlowPersonalScorePeriod {
	const cumulativeAverage = averageDistance(distances);
	const rows = scoreWeights.map((weight, periodIndex) => {
		const completedDistance = distances[periodIndex] ?? 0;
		const unitScore = scoreUnit(completedDistance, cumulativeAverage);
		return {
			periodIndex,
			completedDistance: rounded(completedDistance),
			cumulativeAverage: rounded(cumulativeAverage),
			unitScore: rounded(unitScore),
			weight,
			weightedScore: rounded(weightedScore(unitScore, weight))
		};
	});

	return {
		rows,
		totalScore: Math.round(rows.reduce((total, row) => total + row.weightedScore, 0))
	};
}

function periodDistances(
	tasks: FlowTask[],
	members: FlowMember[],
	definitions: FlowDefinitions,
	week: FlowWeek,
	member: FlowMember,
	periodIndex: (weekStartISO: string, dateISO: string) => number
): number[] {
	const distances = Array.from({ length: scorePeriodCount }, () => 0);
	for (const task of tasks) {
		const endDate = completedTaskEndDate(task);
		if (!endDate || !scoreParticipantIDs(task, members).includes(member.id)) continue;
		const index = periodIndex(week.startISO, endDate);
		if (index < 0 || index >= distances.length) continue;
		distances[index] += completedDistanceForTask(task, definitions);
	}
	return distances;
}

function completedTaskEndDate(task: FlowTask): string {
	if (!isFlowStatusCompleted(task.status)) return '';
	return task.endDate?.trim() ?? '';
}

function scoreParticipantIDs(task: FlowTask, members: FlowMember[]): string[] {
	const memberIDs = new Set(members.map((member) => member.id));
	const memberIDsByName = members.reduce<Record<string, string[]>>((result, member) => {
		result[member.name] = [...(result[member.name] ?? []), member.id];
		return result;
	}, {});
	const ids = new Set<string>();

	for (const participantID of task.participantIDs) {
		if (memberIDs.has(participantID)) ids.add(participantID);
	}
	for (const name of task.participantNames) {
		const nameIDs = memberIDsByName[name] ?? [];
		if (nameIDs.length === 1 && nameIDs[0]) ids.add(nameIDs[0]);
	}
	return Array.from(ids);
}

function weightedScore(unitScore: number, weight: number): number {
	return unitScore * weight * 100 / totalWeight();
}

function totalWeight(): number {
	return scoreWeights.reduce((total, weight) => total + weight, 0);
}

function scoreUnit(distance: number, baseline: number): number {
	if (baseline === 0) return 0;
	return distance / baseline;
}

function averageDistance(distances: number[]): number {
	if (distances.length === 0) return 0;
	return distances.reduce((total, distance) => total + distance, 0) / distances.length;
}

function scoreWeekIndex(weekStartISO: string, dateISO: string): number {
	const weekStart = dateFromISO(weekStartISO);
	const taskWeekStart = startOfISOWeek(dateFromISO(dateISO));
	return Math.trunc((weekStart.getTime() - taskWeekStart.getTime()) / 604_800_000);
}

function scoreMonthIndex(weekStartISO: string, dateISO: string): number {
	const monthStart = dateFromISO(monthStartISO(weekStartISO));
	const taskDate = dateFromISO(dateISO);
	return (monthStart.getUTCFullYear() - taskDate.getUTCFullYear()) * 12 + monthStart.getUTCMonth() - taskDate.getUTCMonth();
}

function startOfISOWeek(date: Date): Date {
	const dayOffset = (date.getUTCDay() + 6) % 7;
	const monday = new Date(date);
	monday.setUTCDate(date.getUTCDate() - dayOffset);
	return monday;
}

function monthStartISO(value: string): string {
	const date = dateFromISO(value);
	return dateToISO(new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1)));
}

function dateFromISO(value: string): Date {
	return new Date(`${value}T00:00:00.000Z`);
}

function dateToISO(date: Date): string {
	return date.toISOString().slice(0, 10);
}

function rounded(value: number): number {
	return Math.round(value * 1000) / 1000;
}

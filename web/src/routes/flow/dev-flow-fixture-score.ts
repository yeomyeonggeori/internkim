// Dev Flow mock의 구성원 점수 계산을 담당합니다.
import { isFlowStatusCompleted } from './flow-status';
import { completedDistanceForTask } from './report/flow-report-distance';
import type { FlowDefinitions, FlowMember, FlowTask, FlowWeek } from './flow-types';

const scorePeriodCount = 5;
const scoreWeights = [1.5, 1.4, 1.3, 1.2, 1.1] as const;

export function buildDevFlowMemberScores(tasks: FlowTask[], members: FlowMember[], definitions: FlowDefinitions, week: FlowWeek): Record<string, number> {
	const weeklyDistances = initializedMemberPeriods(members);
	const monthlyDistances = initializedMemberPeriods(members);

	for (const task of tasks) {
		const endDate = completedTaskEndDate(task);
		if (!endDate) continue;

		const distance = completedDistanceForTask(task, definitions);
		if (distance <= 0) continue;

		addScoreDistance(weeklyDistances, scoreWeekIndex(week.startISO, endDate), task, members, distance);
		addScoreDistance(monthlyDistances, scoreMonthIndex(week.startISO, endDate), task, members, distance);
	}

	return Object.fromEntries(
		members.map((member) => {
			const weeklyScore = weightedScore(weeklyDistances[member.name] ?? []);
			const monthlyScore = weightedScore(monthlyDistances[member.name] ?? []);
			return [member.name, Math.round((weeklyScore + monthlyScore) / 2)];
		})
	);
}

export function totalDevFlowMemberScore(scores: Record<string, number>): number {
	return Object.values(scores).reduce((total, score) => total + score, 0);
}

function initializedMemberPeriods(members: FlowMember[]): Record<string, number[]> {
	return Object.fromEntries(members.map((member) => [member.name, Array.from({ length: scorePeriodCount }, () => 0)]));
}

function addScoreDistance(periodDistances: Record<string, number[]>, index: number, task: FlowTask, members: FlowMember[], distance: number): void {
	if (index < 0 || index >= scorePeriodCount) return;

	for (const participantName of scoreParticipantNames(task, members)) {
		const distances = periodDistances[participantName];
		if (!distances) continue;
		distances[index] += distance;
	}
}

function scoreParticipantNames(task: FlowTask, members: FlowMember[]): string[] {
	const memberNamesByID = new Map(members.map((member) => [member.id, member.name]));
	const names = new Set<string>();

	for (const participantID of task.participantIDs) {
		const name = memberNamesByID.get(participantID);
		if (name) names.add(name);
	}
	for (const name of task.participantNames) {
		if (name) names.add(name);
	}
	return Array.from(names);
}

function completedTaskEndDate(task: FlowTask): string {
	if (!isFlowStatusCompleted(task.status)) return '';
	return task.endDate?.trim() ?? '';
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

function weightedScore(distances: number[]): number {
	if (distances.length === 0) return 0;

	let weightedTotal = 0;
	let weightTotal = 0;
	for (const [index, weight] of scoreWeights.entries()) {
		if (index >= distances.length) break;
		weightedTotal += scoreUnit(distances[index] ?? 0, distances.slice(index)) * weight;
		weightTotal += weight;
	}
	if (weightTotal === 0) return 0;
	return (weightedTotal / weightTotal) * 100;
}

function scoreUnit(distance: number, comparisonDistances: number[]): number {
	const average = averageDistance(comparisonDistances);
	if (average === 0) return 0;
	return distance / average;
}

function averageDistance(distances: number[]): number {
	if (distances.length === 0) return 0;
	return distances.reduce((total, distance) => total + distance, 0) / distances.length;
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

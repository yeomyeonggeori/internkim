import { isTaskStatusCompleted } from './task-status';
import { completedDistanceForTask } from './report/task-report-distance';
import type { TaskDefinitions, TaskMember, TaskMemberScoreDetail, Task, TaskWeek } from './task-types';

const scorePeriodCount = 5;
const scoreWeights = [1.5, 1.4, 1.3, 1.2, 1.1] as const;

export function buildDevTaskMemberScores(tasks: Task[], members: TaskMember[], definitions: TaskDefinitions, week: TaskWeek): Record<string, number> {
	return currentDevTaskMemberScores(buildDevTaskMemberScoreDetails(tasks, members, definitions, week));
}

export function buildDevTaskMemberScoreDetails(
	tasks: Task[],
	members: TaskMember[],
	definitions: TaskDefinitions,
	week: TaskWeek
): Record<string, TaskMemberScoreDetail> {
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
			const weeklyScore = weightedScore(weeklyDistances[member.id] ?? []);
			const monthlyScore = weightedScore(monthlyDistances[member.id] ?? []);
			return [
				member.id,
				{
					weeklyScore: Math.round(weeklyScore),
					monthlyScore: Math.round(monthlyScore),
					currentScore: Math.round((weeklyScore + monthlyScore) / 2)
				}
			];
		})
	);
}

export function currentDevTaskMemberScores(details: Record<string, TaskMemberScoreDetail>): Record<string, number> {
	return Object.fromEntries(Object.entries(details).map(([memberID, detail]) => [memberID, detail.currentScore]));
}

export function totalDevTaskMemberScore(scores: Record<string, number>): number {
	return Object.values(scores).reduce((total, score) => total + score, 0);
}

function initializedMemberPeriods(members: TaskMember[]): Record<string, number[]> {
	return Object.fromEntries(members.map((member) => [member.id, Array.from({ length: scorePeriodCount }, () => 0)]));
}

function addScoreDistance(periodDistances: Record<string, number[]>, index: number, task: Task, members: TaskMember[], distance: number): void {
	if (index < 0 || index >= scorePeriodCount) return;

	for (const participantID of scoreParticipantIDs(task, members)) {
		const distances = periodDistances[participantID];
		if (!distances) continue;
		distances[index] += distance;
	}
}

function scoreParticipantIDs(task: Task, members: TaskMember[]): string[] {
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

function completedTaskEndDate(task: Task): string {
	if (!isTaskStatusCompleted(task.status)) return '';
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

	const baseline = averageDistance(distances);
	let weightedTotal = 0;
	let weightTotal = 0;
	for (const [index, weight] of scoreWeights.entries()) {
		if (index >= distances.length) break;
		weightedTotal += scoreUnit(distances[index] ?? 0, baseline) * weight;
		weightTotal += weight;
	}
	if (weightTotal === 0) return 0;
	return (weightedTotal / weightTotal) * 100;
}

function scoreUnit(distance: number, baseline: number): number {
	if (baseline === 0) return 0;
	return distance / baseline;
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

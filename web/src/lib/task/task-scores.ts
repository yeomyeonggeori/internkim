import { taskSizes } from '$lib/task/task-sizes';
import type { TaskMemberScoreDetail, Task } from '../../routes/task/task-types';

const scorePeriodCount = 5;
const periodWeights = [1.5, 1.4, 1.3, 1.2, 1.1];
const completedStatus = 'completed';
const inactiveStatuses = new Set(['rejected', 'stopped']);

const distanceBySizeName = new Map(taskSizes().map((size) => [size.name.toUpperCase(), size.distanceKm]));

export type MemberTaskTally = {
	distance: number;
	activeTaskCount: number;
	completeTaskCount: number;
};

export function memberScoreDetails(
	tasks: Task[],
	memberIDs: string[],
	weekStart: Date
): Record<string, TaskMemberScoreDetail> {
	const weeklyDistances = zeroedPeriods(memberIDs);
	const monthlyDistances = zeroedPeriods(memberIDs);
	const monthStart = startOfMonth(weekStart);

	for (const task of tasks) {
		const completedOn = completedEndDate(task);
		if (!completedOn) continue;
		const distance = distanceOfSize(task.size);
		if (distance <= 0) continue;
		addDistance(weeklyDistances, weekIndex(weekStart, completedOn), task.participantIDs, distance);
		addDistance(monthlyDistances, monthIndex(monthStart, completedOn), task.participantIDs, distance);
	}

	const details: Record<string, TaskMemberScoreDetail> = {};
	for (const memberID of memberIDs) {
		const weeklyScore = weightedScore(weeklyDistances.get(memberID) ?? []);
		const monthlyScore = weightedScore(monthlyDistances.get(memberID) ?? []);
		details[memberID] = {
			weeklyScore: Math.round(weeklyScore),
			monthlyScore: Math.round(monthlyScore),
			currentScore: Math.round((weeklyScore + monthlyScore) / 2)
		};
	}
	return details;
}

export function currentScoresOf(details: Record<string, TaskMemberScoreDetail>): Record<string, number> {
	return Object.fromEntries(Object.entries(details).map(([memberID, detail]) => [memberID, detail.currentScore]));
}

export function totalScoreOf(scores: Record<string, number>): number {
	return Object.values(scores).reduce((total, score) => total + score, 0);
}

export function memberTaskTallies(tasks: Task[], memberIDs: string[]): Record<string, MemberTaskTally> {
	const tallies: Record<string, MemberTaskTally> = {};
	for (const memberID of memberIDs) {
		tallies[memberID] = { distance: 0, activeTaskCount: 0, completeTaskCount: 0 };
	}
	for (const task of tasks) {
		const isCompleted = Boolean(completedEndDate(task));
		const distance = isCompleted ? distanceOfSize(task.size) : 0;
		for (const memberID of new Set(task.participantIDs)) {
			const tally = tallies[memberID];
			if (!tally) continue;
			if (task.status === completedStatus) tally.completeTaskCount += 1;
			else if (!inactiveStatuses.has(task.status)) tally.activeTaskCount += 1;
			tally.distance += distance;
		}
	}
	return tallies;
}

export function startOfISOWeek(date: Date): Date {
	const start = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
	start.setUTCDate(start.getUTCDate() - ((start.getUTCDay() + 6) % 7));
	return start;
}

function zeroedPeriods(memberIDs: string[]): Map<string, number[]> {
	return new Map(memberIDs.map((memberID) => [memberID, new Array<number>(scorePeriodCount).fill(0)]));
}

function addDistance(
	periodDistances: Map<string, number[]>,
	index: number,
	participantIDs: string[],
	distance: number
): void {
	if (index < 0 || index >= scorePeriodCount) return;
	for (const memberID of new Set(participantIDs)) {
		const distances = periodDistances.get(memberID);
		if (!distances) continue;
		distances[index] += distance;
	}
}

function weightedScore(distances: number[]): number {
	if (distances.length === 0) return 0;
	const baseline = averageOf(distances);
	if (baseline === 0) return 0;
	let weightedTotal = 0;
	let weightTotal = 0;
	for (const [index, weight] of periodWeights.entries()) {
		if (index >= distances.length) break;
		weightedTotal += (distances[index] / baseline) * weight;
		weightTotal += weight;
	}
	if (weightTotal === 0) return 0;
	return (weightedTotal / weightTotal) * 100;
}

function averageOf(values: number[]): number {
	if (values.length === 0) return 0;
	return values.reduce((total, value) => total + value, 0) / values.length;
}

function distanceOfSize(sizeName: string): number {
	return distanceBySizeName.get(sizeName.trim().toUpperCase()) ?? 0;
}

function completedEndDate(task: Task): Date | null {
	if (task.status !== completedStatus) return null;
	const day = (task.endDate ?? '').trim();
	if (!day) return null;
	const parsed = new Date(`${day}T00:00:00Z`);
	return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function weekIndex(weekStart: Date, completedOn: Date): number {
	const millisecondsPerWeek = 7 * 24 * 60 * 60 * 1000;
	return Math.round((weekStart.getTime() - startOfISOWeek(completedOn).getTime()) / millisecondsPerWeek);
}

function monthIndex(monthStart: Date, completedOn: Date): number {
	return (
		(monthStart.getUTCFullYear() - completedOn.getUTCFullYear()) * 12 +
		(monthStart.getUTCMonth() - completedOn.getUTCMonth())
	);
}

function startOfMonth(date: Date): Date {
	return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

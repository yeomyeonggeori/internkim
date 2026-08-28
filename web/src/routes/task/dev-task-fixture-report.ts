import { dateOffset, daysInMonth, monthStartISO } from './dev-task-fixture-date';
import { isTaskStatusCompleted } from './task-status';
import { taskTeamDistance } from './report/task-report-distance';
import type { TaskDefinitions, TaskMember, Task, TaskWeek } from './task-types';
import type { TaskReportSnapshot, TaskReportTrend } from './report/task-report-data';

export function buildDevTaskReportSnapshot(
	tasks: Task[],
	members: TaskMember[],
	definitions: TaskDefinitions,
	week: TaskWeek
): TaskReportSnapshot {
	return {
		weeklyDistanceTrend: buildWeeklyDistanceTrend(tasks, definitions, week),
		monthlyDistanceTrend: buildMonthlyDistanceTrend(tasks, members, definitions, week)
	};
}

function buildWeeklyDistanceTrend(tasks: Task[], definitions: TaskDefinitions, week: TaskWeek): TaskReportTrend {
	const currentValues = cumulativeDailyTeamDistances(tasks, definitions, week.startISO, 7);
	const previousValues = currentValues.map((value, index) => Math.max(0, value - index - 1));

	return {
		labels: Array.from({ length: 7 }, (_, index) => String(index + 1)),
		currentLabel: 'current',
		previousLabel: 'previous',
		currentValues,
		previousValues,
		currentTotal: currentValues.at(-1) ?? 0,
		previousTotal: previousValues.at(-1) ?? 0,
		unit: 'km'
	};
}

function buildMonthlyDistanceTrend(tasks: Task[], members: TaskMember[], definitions: TaskDefinitions, week: TaskWeek): TaskReportTrend {
	const monthStart = monthStartISO(week.startISO);
	const dayCount = daysInMonth(monthStart);
	const currentValues = cumulativeDailyTeamDistances(tasks, definitions, monthStart, dayCount);
	const currentTotal = currentValues.at(-1) ?? 0;
	const baselineTotal = Math.max(currentTotal, members.reduce((total, member) => total + (member.distance ?? member.score ?? 0), 0));
	const previousValues = buildPreviousMonthDistances(dayCount, baselineTotal);

	return {
		labels: Array.from({ length: dayCount }, (_, index) => String(index + 1)),
		currentLabel: 'current',
		previousLabel: 'previous',
		currentValues,
		previousValues,
		currentTotal,
		previousTotal: previousValues.at(-1) ?? 0,
		unit: 'km'
	};
}

function cumulativeDailyTeamDistances(tasks: Task[], definitions: TaskDefinitions, startISO: string, dayCount: number): number[] {
	const dailyDistances = Array.from({ length: dayCount }, () => 0);
	for (const task of tasks) {
		const dayIndex = dateOffset(startISO, distanceDateForTask(task));
		if (dayIndex < 0 || dayIndex >= dayCount) continue;
		dailyDistances[dayIndex] += taskTeamDistance(task, definitions);
	}

	let runningTotal = 0;
	return dailyDistances.map((distance) => {
		runningTotal += distance;
		return runningTotal;
	});
}

function buildPreviousMonthDistances(dayCount: number, currentTotal: number): number[] {
	const targetTotal = Math.max(0, currentTotal - Math.max(4, Math.round(currentTotal * 0.18)));
	let runningTotal = 0;
	return Array.from({ length: dayCount }, (_, index) => {
		const isWorkdayLike = index % 7 !== 5 && index % 7 !== 6;
		if (isWorkdayLike && runningTotal < targetTotal) {
			runningTotal = Math.min(targetTotal, runningTotal + Math.max(1, Math.round(targetTotal / Math.max(8, dayCount - 8))));
		}
		return runningTotal;
	});
}

function distanceDateForTask(task: Task): string {
	if (!isTaskStatusCompleted(task.status)) return '';
	return task.endDate || '';
}

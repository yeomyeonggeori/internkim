import { completedDistanceForTask, taskTeamDistance } from './report/flow-report-distance';
import {
	addDays,
	buildFlowWeek,
	dateOffset,
	daysInMonth,
	monthStartISO,
	weekOffsetFromBaseline
} from './dev-flow-fixture-date';
import {
	devFlowBaselineWeekStartISO,
	devFlowFutureTaskSpecs,
	devFlowMembers,
	devFlowSizes,
	devFlowStatuses,
	devFlowTaskSpecs,
	devFlowTypes,
	type DevFlowTaskSpec
} from './dev-flow-fixture-data';
import type { FlowDefinitions, FlowMember, FlowMetrics, FlowSummary, FlowTask, FlowWeek } from './flow-types';
import type { FlowReportSnapshot, FlowReportTrend } from './report/flow-report-data';

type DevFlowMetrics = FlowMetrics & {
	totalDistance: number;
	totalScore: number;
	memberDistances: Record<string, number>;
	memberScores: Record<string, number>;
};

export type DevFlowSummary = Omit<FlowSummary, 'metrics' | 'report'> & {
	metrics: DevFlowMetrics;
	report: FlowReportSnapshot;
};

export function createDevFlowSummary(weekCode: string | null | undefined, currentUserEmail = 'admin@example.com'): DevFlowSummary {
	const week = buildFlowWeek(weekCode);
	const tasks = createFixtureTasks(week);
	const definitions = { categories: ['여명거리', '김인턴'], types: devFlowTypes, sizes: devFlowSizes };
	const members = calculateMemberDistances(devFlowMembers, tasks, definitions);

	return {
		week,
		members,
		tasks,
		metrics: buildMetrics(tasks, definitions),
		definitions,
		report: buildReportSnapshot(tasks, members, definitions, week),
		statusOptions: devFlowStatuses,
		currentUserEmail,
		currentUserName: memberNameForEmail(members, currentUserEmail),
		isAdmin: true,
		source: 'dev-mock'
	};
}

function createFixtureTasks(week: FlowWeek): FlowTask[] {
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
		if (task.status === '완료') return { ...task, size: completedSizes[(index + weekOffset) % completedSizes.length] ?? task.size };
		if (task.status === '진행') return { ...task, size: progressSizes[(index + weekOffset) % progressSizes.length] ?? task.size };
		return task;
	});
}

function taskFromSpec(spec: DevFlowTaskSpec, week: FlowWeek): FlowTask {
	const owner = memberByID(spec.ownerID);
	const participants = spec.participantIDs.map(memberByID);

	return {
		id: spec.id,
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
		startDate: addDays(week.startISO, spec.startOffset),
		endDate: spec.endOffset > 0 ? addDays(week.startISO, spec.endOffset) : '',
		weekCode: week.code,
		flag: spec.status === '중단' || spec.status === '일시정지' ? 1 : 0,
		requestReason: spec.requestReason ?? '',
		decisionReason: ''
	};
}

function buildMetrics(tasks: FlowTask[], definitions: FlowDefinitions): DevFlowMetrics {
	const metrics: DevFlowMetrics = {
		totalTasks: 0,
		completedTasks: 0,
		requestedTasks: 0,
		pausedTasks: 0,
		stoppedTasks: 0,
		totalDistance: 0,
		totalScore: 0,
		statusCounts: {},
		businessCounts: {},
		typeCounts: {},
		memberDistances: {},
		memberScores: {}
	};

	for (const task of tasks) {
		metrics.totalTasks += 1;
		increment(metrics.statusCounts, task.status, 1);
		increment(metrics.businessCounts, task.business, 1);
		increment(metrics.typeCounts, task.type, 1);
		if (task.status === '완료') metrics.completedTasks += 1;
		if (task.status === '요청') metrics.requestedTasks += 1;
		if (task.status === '일시정지') metrics.pausedTasks += 1;
		if (task.status === '중단') metrics.stoppedTasks += 1;

		const distance = completedDistanceForTask(task, definitions);
		metrics.totalDistance += distance;
		metrics.totalScore += distance;
		for (const participantName of task.participantNames) {
			increment(metrics.memberDistances, participantName, distance);
			increment(metrics.memberScores, participantName, distance);
		}
	}

	return metrics;
}

function buildReportSnapshot(tasks: FlowTask[], members: FlowMember[], definitions: FlowDefinitions, week: FlowWeek): FlowReportSnapshot {
	return {
		weeklyDistanceTrend: buildWeeklyDistanceTrend(tasks, definitions, week),
		monthlyDistanceTrend: buildMonthlyDistanceTrend(tasks, members, definitions, week)
	};
}

function buildWeeklyDistanceTrend(tasks: FlowTask[], definitions: FlowDefinitions, week: FlowWeek): FlowReportTrend {
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

function buildMonthlyDistanceTrend(tasks: FlowTask[], members: FlowMember[], definitions: FlowDefinitions, week: FlowWeek): FlowReportTrend {
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

function cumulativeDailyTeamDistances(tasks: FlowTask[], definitions: FlowDefinitions, startISO: string, dayCount: number): number[] {
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

function calculateMemberDistances(members: FlowMember[], tasks: FlowTask[], definitions: FlowDefinitions): FlowMember[] {
	return members.map((member) => {
		const memberTasks = tasks.filter((task) => task.participantIDs.includes(member.id));
		const distance = memberTasks.reduce((total, task) => total + completedDistanceForTask(task, definitions), 0);
		return {
			...member,
			distance,
			score: distance,
			activeTaskCount: memberTasks.filter((task) => task.status !== '완료' && task.status !== '기각' && task.status !== '중단').length,
			completeTaskCount: memberTasks.filter((task) => task.status === '완료').length
		};
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

function distanceDateForTask(task: FlowTask): string {
	if (task.status !== '완료') return '';
	return task.endDate || '';
}

function memberByID(id: string): FlowMember {
	const found = devFlowMembers.find((member) => member.id === id);
	if (!found) throw new Error(`unknown dev flow member: ${id}`);
	return found;
}

function memberNameForEmail(members: FlowMember[], email: string): string {
	return members.find((member) => member.email === email)?.name ?? email.split('@')[0] ?? '';
}

function increment(record: Record<string, number>, key: string, amount: number): void {
	record[key] = (record[key] ?? 0) + amount;
}

import { buildFlowWeek } from './dev-flow-fixture-date';
import { devFlowMembers, devFlowSizes, devFlowStatuses, devFlowTypes } from './dev-flow-fixture-data';
import { buildDevFlowMemberScoreDetails, currentDevFlowMemberScores } from './dev-flow-fixture-score';
import { createDevFlowScoreTasks } from './dev-flow-fixture-score-tasks';
import {
	applyDevFlowMemberScores,
	buildDevFlowMetrics,
	calculateDevFlowMemberDistances,
	type DevFlowMetrics
} from './dev-flow-fixture-metrics';
import { buildDevFlowReportSnapshot } from './dev-flow-fixture-report';
import { createFixtureTasks } from './dev-flow-fixture-tasks';
import {
	devPopupOverflowCompletedTaskTitles,
	devPopupOverflowDate,
	devPopupOverflowDisplayName
} from '../../../dev-popup-overflow-fixture';
import type { FlowDefinitions, FlowMember, FlowState, FlowSummary, FlowTask, FlowWeek, FlowWeeklySummary } from './flow-types';
import type { FlowReportSnapshot } from './report/flow-report-data';

export type DevFlowSummary = Omit<FlowSummary, 'metrics' | 'report'> & {
	metrics: DevFlowMetrics;
	report: FlowReportSnapshot;
};

export type DevFlowWeeklySummary = Omit<FlowWeeklySummary, 'metrics' | 'report'> & {
	metrics: DevFlowMetrics;
	report: FlowReportSnapshot;
};

export type DevFlowState = Omit<FlowState, 'metrics'> & {
	metrics: DevFlowMetrics;
};

export function createDevFlowSummary(weekCode: string | null | undefined, currentUserEmail = 'admin@example.com'): DevFlowSummary {
	const state = createDevFlowState(currentUserEmail);
	const weeklySummary = createDevFlowWeeklySummary(weekCode);

	return {
		week: weeklySummary.week,
		currentWeek: state.currentWeek,
		members: state.members,
		tasks: state.tasks,
		weeklyTasks: weeklySummary.weeklyTasks,
		metrics: {
			...weeklySummary.metrics,
			memberScores: state.metrics.memberScores,
			memberScoreDetails: state.metrics.memberScoreDetails,
			totalScore: state.metrics.totalScore
		},
		definitions: state.definitions,
		report: weeklySummary.report,
		statusOptions: state.statusOptions,
		currentUserEmail: state.currentUserEmail,
		currentUserName: state.currentUserName,
		isAdmin: state.isAdmin,
		source: state.source
	};
}

export function createDevFlowWeeklySummary(weekCode: string | null | undefined): DevFlowWeeklySummary {
	const week = buildFlowWeek(weekCode);
	const currentWeek = buildFlowWeek(undefined);
	const weeklyTasks = createFixtureTasks(week);
	const definitions = createDevFlowDefinitions();

	return {
		week,
		currentWeek,
		weeklyTasks,
		metrics: buildDevFlowMetrics(weeklyTasks, definitions, {}, {}),
		report: buildDevFlowReportSnapshot(weeklyTasks, devFlowMembers, definitions, week),
		source: 'dev-mock'
	};
}

export function createDevFlowState(currentUserEmail = 'admin@example.com'): DevFlowState {
	const currentWeek = buildFlowWeek(undefined);
	const tasks = createGlobalFixtureTasks(currentWeek);
	const definitions = createDevFlowDefinitions();
	const memberScoreDetails = buildDevFlowMemberScoreDetails(createDevFlowScoreTasks(currentWeek, createFixtureTasks), devFlowMembers, definitions, currentWeek);
	const memberScores = currentDevFlowMemberScores(memberScoreDetails);
	const memberDistances = calculateDevFlowMemberDistances(devFlowMembers, tasks, definitions);
	const members = applyDevFlowMemberScores(memberDistances, memberScores);

	return {
		currentWeek,
		members,
		tasks,
		metrics: buildDevFlowMetrics([], definitions, memberScores, memberScoreDetails),
		definitions,
		statusOptions: devFlowStatuses,
		currentUserEmail,
		currentUserName: memberNameForEmail(members, currentUserEmail),
		isAdmin: true,
		source: 'dev-mock'
	};
}

function createDevFlowDefinitions(): FlowDefinitions {
	return { categories: ['여명거리', '김인턴'], types: devFlowTypes, sizes: devFlowSizes };
}

function createGlobalFixtureTasks(currentWeek: FlowWeek): FlowTask[] {
	const weeks = [buildFlowWeek(currentWeek.previous), currentWeek, buildFlowWeek(currentWeek.next)];
	return [
		...weeks.flatMap((week) => createFixtureTasks(week).map((task) => ({ ...task, id: `${week.code}-${task.id}` }))),
		...createPopupOverflowTasks()
	];
}

function memberNameForEmail(members: FlowMember[], email: string): string {
	return members.find((member) => member.email === email)?.name ?? email.split('@')[0] ?? '';
}

function createPopupOverflowTasks(): FlowTask[] {
	return devPopupOverflowCompletedTaskTitles.map((title, index) =>
		popupOverflowTask(`dev-popup-overflow-task-${index + 1}`, title, (index + 1) * 1024)
	);
}

function popupOverflowTask(id: string, content: string, statusRank: number): FlowTask {
	return {
		id,
		ownerID: 'kim-intern',
		ownerName: devPopupOverflowDisplayName,
		participantIDs: ['kim-intern'],
		participantNames: [devPopupOverflowDisplayName],
		business: '김인턴',
		type: '검증',
		content,
		goal: '출결 월간 현황 팝업에서 완료 업무 overflow 상태를 확인한다.',
		size: 'S',
		status: '완료',
		statusRank,
		startDate: devPopupOverflowDate,
		endDate: devPopupOverflowDate,
		createdAt: `${devPopupOverflowDate}T09:00:00Z`,
		weekCode: '26W25',
		flag: 0,
		requestReason: '',
		decisionReason: ''
	};
}

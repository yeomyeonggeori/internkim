import { buildTaskWeek } from './dev-task-fixture-date';
import { devTaskMembers, devTaskStatuses, devTaskTypes } from './dev-task-fixture-data';
import { taskSizes } from '../../lib/task/task-sizes';
import { buildDevTaskMemberScoreDetails, currentDevTaskMemberScores } from './dev-task-fixture-score';
import { createDevTaskScoreTasks } from './dev-task-fixture-score-tasks';
import {
	applyDevTaskMemberScores,
	buildDevTaskMetrics,
	calculateDevTaskMemberDistances,
	type DevTaskMetrics
} from './dev-task-fixture-metrics';
import { buildDevTaskReportSnapshot } from './dev-task-fixture-report';
import { createFixtureTasks } from './dev-task-fixture-tasks';
import {
	devPopupOverflowCompletedTaskTitles,
	devPopupOverflowDate,
	devPopupOverflowDisplayName
} from '../../../dev-popup-overflow-fixture';
import { todayDateInTimeZone } from '../attendance/shared/attendance-date';
import type { TaskDefinitions, TaskMember, TaskState, TaskSummary, Task, TaskWeek, TaskWeeklySummary } from './task-types';
import type { TaskReportSnapshot } from './report/task-report-data';

export type DevTaskSummary = Omit<TaskSummary, 'metrics' | 'report'> & {
	metrics: DevTaskMetrics;
	report: TaskReportSnapshot;
};

export type DevTaskWeeklySummary = Omit<TaskWeeklySummary, 'metrics' | 'report'> & {
	metrics: DevTaskMetrics;
	report: TaskReportSnapshot;
};

export type DevTaskState = Omit<TaskState, 'metrics'> & {
	metrics: DevTaskMetrics;
};

export function createDevTaskSummary(weekCode: string | null | undefined, currentUserEmail = 'kim@example.com'): DevTaskSummary {
	const state = createDevTaskState(currentUserEmail);
	const weeklySummary = createDevTaskWeeklySummary(weekCode);

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

export function createDevTaskWeeklySummary(weekCode: string | null | undefined): DevTaskWeeklySummary {
	const week = buildTaskWeek(weekCode);
	const currentWeek = buildTaskWeek(undefined);
	const weeklyTasks = createFixtureTasks(week);
	const definitions = createDevTaskDefinitions();

	return {
		week,
		currentWeek,
		weeklyTasks,
		metrics: buildDevTaskMetrics(weeklyTasks, definitions, {}, {}),
		report: buildDevTaskReportSnapshot(weeklyTasks, devTaskMembers, definitions, week),
		source: 'dev-mock'
	};
}

export function createDevTaskState(currentUserEmail = 'kim@example.com', locale: 'ko' | 'en' = 'ko'): DevTaskState {
	const currentWeek = buildTaskWeek(undefined);
	const tasks = createGlobalFixtureTasks(currentWeek);
	const definitions = createDevTaskDefinitions(locale);
	const memberScoreDetails = buildDevTaskMemberScoreDetails(createDevTaskScoreTasks(currentWeek, createFixtureTasks), devTaskMembers, definitions, currentWeek);
	const memberScores = currentDevTaskMemberScores(memberScoreDetails);
	const memberDistances = calculateDevTaskMemberDistances(devTaskMembers, tasks, definitions);
	const members = applyDevTaskMemberScores(memberDistances, memberScores);

	return {
		currentWeek,
		members,
		tasks,
		metrics: buildDevTaskMetrics([], definitions, memberScores, memberScoreDetails),
		definitions,
		statusOptions: devTaskStatuses,
		currentUserEmail,
		currentUserName: memberNameForEmail(members, currentUserEmail),
		isAdmin: true,
		source: 'dev-mock'
	};
}

function createDevTaskDefinitions(locale: 'ko' | 'en' = 'ko'): TaskDefinitions {
	return { categories: ['샘플거리', '김인턴'], types: devTaskTypes, sizes: taskSizes(locale === 'en' ? 'en' : 'ko') };
}

function createGlobalFixtureTasks(currentWeek: TaskWeek): Task[] {
	const weeks = [buildTaskWeek(currentWeek.previous), currentWeek, buildTaskWeek(currentWeek.next)];
	return [
		...weeks.flatMap((week) => createFixtureTasks(week).map((task) => ({
			...task,
			id: `${week.code}-${task.id}`,
			parentTaskID: task.parentTaskID ? `${week.code}-${task.parentTaskID}` : undefined
		}))),
		...createPopupOverflowTasks(),
		...createAttendancePreviewTasks()
	];
}

function memberNameForEmail(members: TaskMember[], email: string): string {
	return members.find((member) => member.email === email)?.name ?? email.split('@')[0] ?? '';
}

function createPopupOverflowTasks(): Task[] {
	return devPopupOverflowCompletedTaskTitles.map((title, index) =>
		popupOverflowTask(`dev-popup-overflow-${index + 1}`, title, (index + 1) * 1024)
	);
}

function createAttendancePreviewTasks(): Task[] {
	const date = todayDateInTimeZone('Asia/Seoul', new Date());
	return [
		attendancePreviewTask('attendance-preview-task-1', '근무 기록 카드 UI 정리', date, 1024),
		attendancePreviewTask('attendance-preview-task-2', '날짜별 근태 편집 흐름 검증', date, 2048)
	];
}

function attendancePreviewTask(id: string, content: string, date: string, statusRank: number): Task {
	return {
		id,
		ownerID: 'kim-intern',
		ownerName: devPopupOverflowDisplayName,
		participantIDs: ['kim-intern', 'designer'],
		participantNames: [devPopupOverflowDisplayName, '이영희'],
		business: '김인턴',
		type: '검증',
		content,
		size: 'S',
		status: 'completed',
		statusRank,
		startDate: date,
		endDate: date,
		createdAt: `${date}T09:00:00+09:00`,
		weekCode: ''
	};
}

function popupOverflowTask(id: string, content: string, statusRank: number): Task {
	return {
		id,
		ownerID: 'kim-intern',
		ownerName: devPopupOverflowDisplayName,
		participantIDs: ['kim-intern'],
		participantNames: [devPopupOverflowDisplayName],
		business: '김인턴',
		type: '검증',
		content,
		size: 'S',
		status: 'completed',
		statusRank,
		startDate: devPopupOverflowDate,
		endDate: devPopupOverflowDate,
		createdAt: `${devPopupOverflowDate}T09:00:00Z`,
		weekCode: '26W25'
	};
}

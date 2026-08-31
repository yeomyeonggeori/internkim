import type { TaskDefinitions, TaskMember, TaskMetrics, TaskSummary, Task } from './task-types';
import { buildTaskReportSections } from './report/task-report-data';
import { taskText } from './text';

type TaskPageText = typeof taskText.ko;

type TaskReportSectionsInput = {
	metrics: TaskMetrics;
	text: TaskPageText;
	summary: TaskSummary | null;
	tasks: Task[];
	members: TaskMember[];
	definitions: TaskDefinitions;
};

export const emptyTaskMetrics: TaskMetrics = {
	totalTasks: 0,
	completedTasks: 0,
	requestedTasks: 0,
	pausedTasks: 0,
	stoppedTasks: 0,
	totalDistance: 0,
	statusCounts: {},
	businessCounts: {},
	typeCounts: {},
	memberDistances: {}
};

export function createTaskReportSections(input: TaskReportSectionsInput) {
	return buildTaskReportSections(input.metrics, {
		emptyLabel: input.text.report.empty,
		sectionLabels: {
			weeklyStatus: {
				title: input.text.report.weeklyStatus,
				description: input.text.report.weeklyStatusDescription
			},
			memberDistance: {
				title: input.text.report.memberDistance,
				description: input.text.report.memberDistanceDescription
			},
			weeklyDistanceTrend: {
				title: input.text.report.weeklyDistanceTrend,
				description: input.text.report.weeklyDistanceTrendDescription
			},
			monthlyDistanceTrend: {
				title: input.text.report.monthlyDistanceTrend,
				description: input.text.report.monthlyDistanceTrendDescription
			},
			businessDistance: {
				title: input.text.report.businessDistance,
				description: input.text.report.businessDistanceDescription
			}
		},
		copy: {
			weekdays: [...input.text.report.weekdays],
			etcLabel: input.text.report.etcLabel,
			memberScoreLabel: input.text.report.memberScoreLabel,
			weeklyScoreLabel: input.text.report.weeklyScoreLabel,
			monthlyScoreLabel: input.text.report.monthlyScoreLabel,
			scoreUnit: input.text.report.scoreUnit,
			teamAverageLabel: input.text.report.teamAverageLabel,
			memberScrollHint: input.text.report.memberScrollHint,
			currentWeekTrend: input.text.report.currentWeekTrend,
			previousWeekTrend: input.text.report.previousWeekTrend,
			currentMonthTrend: input.text.report.currentMonthTrend,
			previousMonthTrend: input.text.report.previousMonthTrend,
			monthlyDayLabelTemplate: input.text.report.monthlyDayLabelTemplate
		},
		report: input.summary?.report,
		tasks: input.tasks,
		members: input.members,
		definitions: input.definitions,
		weekStartISO: input.summary?.week.startISO
	});
}

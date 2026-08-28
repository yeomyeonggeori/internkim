import { buildBusinessDistanceSection } from './task-business-distance-section';
import { buildMemberScoreSection } from './task-member-score-section';
import { averagePositiveValue } from './task-report-section-helpers';
import { buildTrendSection } from './task-trend-section';
import { buildWeeklyTypeDistanceSection } from './task-weekly-type-distance-section';
import type {
	TaskReportDefinitions,
	TaskReportMember,
	TaskReportMemberScoreDetail,
	TaskReportMetrics,
	TaskReportOptions,
	TaskReportSections
} from './task-report-types';

export type {
	TaskBusinessDistanceSection,
	TaskChartSection,
	TaskDailyTypeDistanceSection,
	TaskMemberScoreSection,
	TaskReportChartKind,
	TaskReportCopy,
	TaskReportDefinitions,
	TaskReportItem,
	TaskReportMember,
	TaskReportMemberScoreDetail,
	TaskReportMetrics,
	TaskReportOptions,
	TaskReportRow,
	TaskReportSection,
	TaskReportSectionID,
	TaskReportSectionLabel,
	TaskReportSectionLabels,
	TaskReportSections,
	TaskReportSegment,
	TaskReportSnapshot,
	TaskReportTask,
	TaskReportTone,
	TaskReportTrend,
	TaskTrendSection
} from './task-report-types';

type TaskMetricScores = {
	memberScores: Record<string, number>;
	memberScoreDetails: Record<string, TaskReportMemberScoreDetail>;
};

const emptyDefinitions: TaskReportDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function buildTaskReportSections(metrics: TaskReportMetrics, options: TaskReportOptions): TaskReportSections {
	const definitions = options.definitions ?? emptyDefinitions;
	const tasks = options.tasks ?? [];
	const metricScores = scoresFromMetrics(metrics, options.members ?? []);
	const copy = options.copy;

	return {
		weeklyStatus: buildWeeklyTypeDistanceSection({
			id: 'weeklyStatus',
			labels: options.sectionLabels.weeklyStatus,
			emptyLabel: options.emptyLabel,
			copy,
			tasks,
			definitions,
			weekStartISO: options.weekStartISO
		}),
		memberDistance: buildMemberScoreSection({
			id: 'memberDistance',
			labels: options.sectionLabels.memberDistance,
			emptyLabel: options.emptyLabel,
			copy,
			memberScores: metricScores.memberScores,
			memberScoreDetails: metricScores.memberScoreDetails,
			averageValue: averagePositiveValue(metricScores.memberScores)
		}),
		weeklyDistanceTrend: buildTrendSection({
			id: 'weeklyDistanceTrend',
			labels: options.sectionLabels.weeklyDistanceTrend,
			emptyLabel: options.emptyLabel,
			copy,
			trend: options.report?.weeklyDistanceTrend
		}),
		monthlyDistanceTrend: buildTrendSection({
			id: 'monthlyDistanceTrend',
			labels: options.sectionLabels.monthlyDistanceTrend,
			emptyLabel: options.emptyLabel,
			copy,
			trend: options.report?.monthlyDistanceTrend
		}),
		businessDistance: buildBusinessDistanceSection({
			id: 'businessDistance',
			labels: options.sectionLabels.businessDistance,
			emptyLabel: options.emptyLabel,
			copy,
			tasks,
			definitions
		})
	};
}

function scoresFromMetrics(metrics: TaskReportMetrics, members: TaskReportMember[]): TaskMetricScores {
	const scores = currentScoresFromDetails(metrics.memberScoreDetails) ?? metrics.memberScores ?? metrics.memberDistances ?? {};
	return {
		memberScores: labelMemberScores(scores, members),
		memberScoreDetails: labelMemberScoreDetails(metrics.memberScoreDetails ?? {}, members)
	};
}

function currentScoresFromDetails(details: Record<string, TaskReportMemberScoreDetail> | undefined): Record<string, number> | undefined {
	if (!details) return undefined;
	return Object.fromEntries(Object.entries(details).map(([memberKey, detail]) => [memberKey, detail.currentScore]));
}

function labelMemberScores(scores: Record<string, number>, members: TaskReportMember[]): Record<string, number> {
	const labels = memberScoreLabels(members);
	const values = Object.fromEntries(members.map((member) => [labels.get(member.id) ?? member.name, scores[member.id] ?? scores[member.name] ?? 0]));
	for (const [key, score] of Object.entries(scores)) {
		values[labels.get(key) ?? key] = score;
	}
	return values;
}

function labelMemberScoreDetails(details: Record<string, TaskReportMemberScoreDetail>, members: TaskReportMember[]): Record<string, TaskReportMemberScoreDetail> {
	const labels = memberScoreLabels(members);
	const values = Object.fromEntries(
		members.map((member) => [
			labels.get(member.id) ?? member.name,
			details[member.id] ?? details[member.name] ?? { weeklyScore: 0, monthlyScore: 0, currentScore: 0 }
		])
	);
	for (const [key, detail] of Object.entries(details)) {
		values[labels.get(key) ?? key] = detail;
	}
	return values;
}

function memberScoreLabels(members: TaskReportMember[]): Map<string, string> {
	const nameCounts = members.reduce<Record<string, number>>((counts, member) => {
		counts[member.name] = (counts[member.name] ?? 0) + 1;
		return counts;
	}, {});
	return new Map(members.map((member) => [member.id, nameCounts[member.name] === 1 ? member.name : `${member.name} (${member.id})`]));
}

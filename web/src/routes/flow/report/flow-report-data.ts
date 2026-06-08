import { buildBusinessDistanceSection } from './flow-business-distance-section';
import { buildMemberScoreSection } from './flow-member-score-section';
import { averagePositiveValue } from './flow-report-section-helpers';
import { buildTrendSection } from './flow-trend-section';
import { buildWeeklyTypeDistanceSection } from './flow-weekly-type-distance-section';
import type { FlowReportDefinitions, FlowReportMember, FlowReportMetrics, FlowReportOptions, FlowReportSections } from './flow-report-types';

export type {
	FlowReportChartKind,
	FlowReportCopy,
	FlowReportDefinitions,
	FlowReportItem,
	FlowReportMember,
	FlowReportMetrics,
	FlowReportOptions,
	FlowReportRow,
	FlowReportSection,
	FlowReportSectionID,
	FlowReportSectionLabel,
	FlowReportSectionLabels,
	FlowReportSections,
	FlowReportSegment,
	FlowReportSnapshot,
	FlowReportTask,
	FlowReportTone,
	FlowReportTrend
} from './flow-report-types';

type FlowMetricScores = {
	memberScores: Record<string, number>;
};

const emptyDefinitions: FlowReportDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function buildFlowReportSections(metrics: FlowReportMetrics, options: FlowReportOptions): FlowReportSections {
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

function scoresFromMetrics(metrics: FlowReportMetrics, members: FlowReportMember[]): FlowMetricScores {
	const scores = metrics.memberScores ?? metrics.memberDistances ?? {};
	return {
		memberScores: labelMemberScores(scores, members)
	};
}

function labelMemberScores(scores: Record<string, number>, members: FlowReportMember[]): Record<string, number> {
	const labels = memberScoreLabels(members);
	return Object.fromEntries(Object.entries(scores).map(([key, score]) => [labels.get(key) ?? key, score]));
}

function memberScoreLabels(members: FlowReportMember[]): Map<string, string> {
	const nameCounts = members.reduce<Record<string, number>>((counts, member) => {
		counts[member.name] = (counts[member.name] ?? 0) + 1;
		return counts;
	}, {});
	return new Map(members.map((member) => [member.id, nameCounts[member.name] === 1 ? member.name : `${member.name} (${member.id})`]));
}

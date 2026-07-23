import { buildBusinessDistanceSection } from './flow-business-distance-section';
import { buildMemberScoreSection } from './flow-member-score-section';
import { averagePositiveValue } from './flow-report-section-helpers';
import { buildTrendSection } from './flow-trend-section';
import { buildWeeklyTypeDistanceSection } from './flow-weekly-type-distance-section';
import type {
	FlowReportDefinitions,
	FlowReportMember,
	FlowReportMemberScoreDetail,
	FlowReportMetrics,
	FlowReportOptions,
	FlowReportSections
} from './flow-report-types';

export type {
	FlowBusinessDistanceSection,
	FlowChartSection,
	FlowDailyTypeDistanceSection,
	FlowMemberScoreSection,
	FlowReportChartKind,
	FlowReportCopy,
	FlowReportDefinitions,
	FlowReportItem,
	FlowReportMember,
	FlowReportMemberScoreDetail,
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
	FlowReportTrend,
	FlowTrendSection
} from './flow-report-types';

type FlowMetricScores = {
	memberScores: Record<string, number>;
	memberScoreDetails: Record<string, FlowReportMemberScoreDetail>;
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

function scoresFromMetrics(metrics: FlowReportMetrics, members: FlowReportMember[]): FlowMetricScores {
	const scores = currentScoresFromDetails(metrics.memberScoreDetails) ?? metrics.memberScores ?? metrics.memberDistances ?? {};
	return {
		memberScores: labelMemberScores(scores, members),
		memberScoreDetails: labelMemberScoreDetails(metrics.memberScoreDetails ?? {}, members)
	};
}

function currentScoresFromDetails(details: Record<string, FlowReportMemberScoreDetail> | undefined): Record<string, number> | undefined {
	if (!details) return undefined;
	return Object.fromEntries(Object.entries(details).map(([memberKey, detail]) => [memberKey, detail.currentScore]));
}

function labelMemberScores(scores: Record<string, number>, members: FlowReportMember[]): Record<string, number> {
	const labels = memberScoreLabels(members);
	const values = Object.fromEntries(members.map((member) => [labels.get(member.id) ?? member.name, scores[member.id] ?? scores[member.name] ?? 0]));
	for (const [key, score] of Object.entries(scores)) {
		values[labels.get(key) ?? key] = score;
	}
	return values;
}

function labelMemberScoreDetails(details: Record<string, FlowReportMemberScoreDetail>, members: FlowReportMember[]): Record<string, FlowReportMemberScoreDetail> {
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

function memberScoreLabels(members: FlowReportMember[]): Map<string, string> {
	const nameCounts = members.reduce<Record<string, number>>((counts, member) => {
		counts[member.name] = (counts[member.name] ?? 0) + 1;
		return counts;
	}, {});
	return new Map(members.map((member) => [member.id, nameCounts[member.name] === 1 ? member.name : `${member.name} (${member.id})`]));
}

import { buildBusinessDistanceSection } from './flow-business-distance-section';
import { buildMemberDistanceSection } from './flow-member-distance-section';
import { averagePositiveValue } from './flow-report-section-helpers';
import { buildTrendSection } from './flow-trend-section';
import { buildWeeklyTypeDistanceSection } from './flow-weekly-type-distance-section';
import type { FlowReportDefinitions, FlowReportMetrics, FlowReportOptions, FlowReportSections } from './flow-report-types';

export type {
	FlowReportChartKind,
	FlowReportDefinitions,
	FlowReportItem,
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

type FlowMetricDistances = {
	memberDistances: Record<string, number>;
};

const emptyDefinitions: FlowReportDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function buildFlowReportSections(metrics: FlowReportMetrics, options: FlowReportOptions): FlowReportSections {
	const definitions = options.definitions ?? emptyDefinitions;
	const tasks = options.tasks ?? [];
	const metricDistances = distancesFromMetrics(metrics);

	return {
		weeklyStatus: buildWeeklyTypeDistanceSection({
			id: 'weeklyStatus',
			labels: options.sectionLabels.weeklyStatus,
			emptyLabel: options.emptyLabel,
			tasks,
			definitions,
			weekStartISO: options.weekStartISO
		}),
		memberDistance: buildMemberDistanceSection({
			id: 'memberDistance',
			labels: options.sectionLabels.memberDistance,
			emptyLabel: options.emptyLabel,
			tasks,
			definitions,
			memberDistances: metricDistances.memberDistances,
			averageValue: averagePositiveValue(metricDistances.memberDistances)
		}),
		weeklyDistanceTrend: buildTrendSection({
			id: 'weeklyDistanceTrend',
			labels: options.sectionLabels.weeklyDistanceTrend,
			emptyLabel: options.emptyLabel,
			trend: options.report?.weeklyDistanceTrend
		}),
		monthlyDistanceTrend: buildTrendSection({
			id: 'monthlyDistanceTrend',
			labels: options.sectionLabels.monthlyDistanceTrend,
			emptyLabel: options.emptyLabel,
			trend: options.report?.monthlyDistanceTrend
		}),
		businessDistance: buildBusinessDistanceSection({
			id: 'businessDistance',
			labels: options.sectionLabels.businessDistance,
			emptyLabel: options.emptyLabel,
			tasks,
			definitions
		})
	};
}

function distancesFromMetrics(metrics: FlowReportMetrics): FlowMetricDistances {
	return {
		memberDistances: metrics.memberScores
	};
}

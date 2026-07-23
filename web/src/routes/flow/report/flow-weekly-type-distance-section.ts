import { taskTeamDistance, weeklyTaskDayIndex } from './flow-report-distance';
import { emptyTrend, incrementNested, percentage, sortedTypeDistances, typeIndex } from './flow-report-section-helpers';
import type { FlowDailyTypeDistanceSection, FlowReportCopy, FlowReportDefinitions, FlowReportRow, FlowReportSectionLabel, FlowReportTask } from './flow-report-types';

type BuildWeeklyTypeDistanceSectionInput = {
	id: FlowDailyTypeDistanceSection['id'];
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	copy: FlowReportCopy;
	tasks: FlowReportTask[];
	definitions: FlowReportDefinitions;
	weekStartISO?: string;
};

export function buildWeeklyTypeDistanceSection(input: BuildWeeklyTypeDistanceSectionInput): FlowDailyTypeDistanceSection {
	const weeklyDayLabels = normalizedWeekdays(input.copy.weekdays);
	const dailyTypeDistances = new Map<string, Map<string, number>>();
	for (const label of weeklyDayLabels) {
		dailyTypeDistances.set(label, new Map<string, number>());
	}

	for (const task of input.tasks) {
		const dayIndex = weeklyTaskDayIndex(task, input.weekStartISO);
		if (dayIndex < 0 || dayIndex >= weeklyDayLabels.length) continue;

		const distance = taskTeamDistance(task, input.definitions);
		if (distance <= 0) continue;

		incrementNested(dailyTypeDistances, weeklyDayLabels[dayIndex], task.type, distance);
	}

	const typeOrder = [...input.definitions.types, input.copy.fallbackType];
	const rows = weeklyDayLabels.map((label) => dailyTypeDistanceRow(label, dailyTypeDistances.get(label) ?? new Map<string, number>(), typeOrder));
	const total = rows.reduce((sum, row) => sum + row.total, 0);
	const typeTotals = totalTypeDistances(rows, typeOrder);

	return {
		id: input.id,
		chartKind: 'dailyTypeStacked',
		title: input.labels.title,
		description: input.labels.description,
		unit: 'km',
		total,
		maxValue: 100,
		averageValue: 0,
		alertValue: 0,
		emptyLabel: input.emptyLabel,
		teamAverageLabel: input.copy.teamAverageLabel,
		memberScrollHint: input.copy.memberScrollHint,
		items: typeTotals.map(([typeName, value]) => ({
			label: typeName,
			description: '',
			value,
			percent: percentage(value, total),
			tone: 'type',
			colorIndex: typeIndex(typeName, typeOrder)
		})),
		rows: rows.map((row) => ({ ...row, percent: percentage(row.total, total) })),
		trend: emptyTrend('km')
	};
}

function normalizedWeekdays(weekdays: string[]): string[] {
	return weekdays.length === 7 ? weekdays : ['1', '2', '3', '4', '5', '6', '7'];
}

function dailyTypeDistanceRow(label: string, typeDistances: Map<string, number>, typeOrder: string[]): FlowReportRow {
	const total = Array.from(typeDistances.values()).reduce((sum, value) => sum + value, 0);
	return {
		label,
		total,
		percent: 0,
		segments: sortedTypeDistances(typeDistances, typeOrder).map(([typeName, value]) => ({
			label: typeName,
			value,
			percent: percentage(value, total),
			tone: 'type' as const,
			colorIndex: typeIndex(typeName, typeOrder)
		}))
	};
}

function totalTypeDistances(rows: FlowReportRow[], typeOrder: string[]): [string, number][] {
	const totals = new Map<string, number>();
	for (const row of rows) {
		for (const segment of row.segments) {
			totals.set(segment.label, (totals.get(segment.label) ?? 0) + segment.value);
		}
	}
	return sortedTypeDistances(totals, typeOrder);
}

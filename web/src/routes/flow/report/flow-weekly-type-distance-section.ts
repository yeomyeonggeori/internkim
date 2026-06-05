// Flow 주간 업무 종류 거리 섹션을 계산합니다.
import { taskTeamDistance, weeklyTaskDayIndex } from './flow-report-distance';
import { emptyTrend, incrementNested, percentage, sortedTypeDistances, typeIndex } from './flow-report-section-helpers';
import type { FlowReportDefinitions, FlowReportRow, FlowReportSection, FlowReportSectionID, FlowReportSectionLabel, FlowReportTask } from './flow-report-types';

type BuildWeeklyTypeDistanceSectionInput = {
	id: FlowReportSectionID;
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	tasks: FlowReportTask[];
	definitions: FlowReportDefinitions;
	weekStartISO?: string;
};

const weeklyDayLabels = ['월', '화', '수', '목', '금', '토', '일'];

export function buildWeeklyTypeDistanceSection(input: BuildWeeklyTypeDistanceSectionInput): FlowReportSection {
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

	const typeOrder = [...input.definitions.types, '기타'];
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

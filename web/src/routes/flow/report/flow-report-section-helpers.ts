import type { FlowReportTrend } from './flow-report-types';

export function percentage(value: number, total: number): number {
	if (total <= 0) return 0;
	return Math.round((value / total) * 100);
}

export function averagePositiveValue(values: Record<string, number>): number {
	const positiveValues = Object.values(values).filter((value) => value > 0);
	if (positiveValues.length === 0) return 0;
	const total = positiveValues.reduce((sum, value) => sum + value, 0);
	return Math.round(total / positiveValues.length);
}

export function emptyTrend(unit: string): FlowReportTrend {
	return {
		labels: [],
		currentLabel: '',
		previousLabel: '',
		currentValues: [],
		previousValues: [],
		currentTotal: 0,
		previousTotal: 0,
		unit
	};
}

export function incrementNested(values: Map<string, Map<string, number>>, rowKey: string, columnKey: string, amount: number): void {
	const columns = values.get(rowKey) ?? new Map<string, number>();
	columns.set(columnKey, (columns.get(columnKey) ?? 0) + amount);
	values.set(rowKey, columns);
}

export function typeIndex(typeName: string, typeOrder: string[]): number {
	const index = typeOrder.indexOf(typeName);
	return index === -1 ? typeOrder.length : index;
}

export function sortedTypeDistances(typeDistances: Map<string, number>, typeOrder: string[]): [string, number][] {
	return Array.from(typeDistances.entries())
		.filter(([, value]) => value > 0)
		.sort((left, right) => {
			const valueDifference = right[1] - left[1];
			if (valueDifference !== 0) return valueDifference;
			return typeIndex(left[0], typeOrder) - typeIndex(right[0], typeOrder);
		});
}

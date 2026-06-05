import type { ChartMode } from '../attendance-context.svelte';
import { eachDayOfMonth, isoWeekStart } from './attendance-date';

export type DailyValue = { date: string; value: number };
export type ChartPoint = { key: string; label: string; value: number };
export type PlottedPoint = { x: number; y: number; point: ChartPoint };

export type WorkTimeChartLabels = {
	weekLabelTemplate: string;
	total: string;
};

export const VIEW_WIDTH = 1000;
export const VIEW_HEIGHT = 220;
export const PADDING_X = 24;
export const PADDING_TOP = 16;
export const PADDING_BOTTOM = 28;

const innerWidth = VIEW_WIDTH - PADDING_X * 2;
const innerHeight = VIEW_HEIGHT - PADDING_TOP - PADDING_BOTTOM;

export const baselineY = VIEW_HEIGHT - PADDING_BOTTOM;

export function buildSeries(
	month: string,
	dailyValues: DailyValue[],
	mode: ChartMode,
	labels: WorkTimeChartLabels
): ChartPoint[] {
	if (!month) return [];
	const valueByDate = new Map(dailyValues.map((dailyValue) => [dailyValue.date, dailyValue.value]));
	const dailyPoints = eachDayOfMonth(month).map((date) => ({ date, value: valueByDate.get(date) ?? 0 }));

	if (mode === 'day') {
		return dailyPoints.map((dailyPoint) => ({
			key: dailyPoint.date,
			label: dailyPoint.date.slice(8, 10),
			value: dailyPoint.value
		}));
	}
	if (mode === 'week') {
		return buildWeeklySeries(dailyPoints, labels.weekLabelTemplate);
	}
	const total = dailyPoints.reduce((sum, dailyPoint) => sum + dailyPoint.value, 0);
	return [{ key: 'total', label: labels.total, value: total }];
}

function buildWeeklySeries(
	dailyPoints: { date: string; value: number }[],
	weekLabelTemplate: string
): ChartPoint[] {
	const buckets = new Map<string, number>();
	for (const dailyPoint of dailyPoints) {
		const key = isoWeekStart(dailyPoint.date);
		buckets.set(key, (buckets.get(key) ?? 0) + dailyPoint.value);
	}
	return [...buckets.entries()]
		.sort((left, right) => left[0].localeCompare(right[0]))
		.map(([key, value], index) => ({
			key,
			label: weekLabelTemplate.replace('{index}', String(index + 1)),
			value
		}));
}

export function plotPoints(points: ChartPoint[], maxValue: number): PlottedPoint[] {
	const count = points.length;
	return points.map((point, index) => {
		const x = count === 1 ? PADDING_X + innerWidth / 2 : PADDING_X + (innerWidth * index) / (count - 1);
		const ratio = point.value / maxValue;
		const y = baselineY - ratio * innerHeight;
		return { x, y, point };
	});
}

export function buildAreaPath(points: PlottedPoint[]): string {
	if (!points.length) return '';
	const head = `M ${points[0].x.toFixed(1)} ${baselineY}`;
	const top = points.map((point) => `L ${point.x.toFixed(1)} ${point.y.toFixed(1)}`).join(' ');
	const tail = `L ${points[points.length - 1].x.toFixed(1)} ${baselineY} Z`;
	return `${head} ${top} ${tail}`;
}

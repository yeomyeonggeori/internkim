import type { ChartMode } from '../attendance-context.svelte';
import { addDays, eachDayOfMonth, utcDateKey } from './attendance-date';

export type DailyValue = { date: string; value: number };
export type ChartPoint = { key: string; label: string; tooltipLabel: string; value: number };
export type PlottedPoint = { x: number; y: number; point: ChartPoint };

export type WorkTimeChartLabels = {
	weekLabelTemplate: string;
	monthLabelTemplate: string;
	total: string;
	weekdaySunday: string;
	weekdayMonday: string;
	weekdayTuesday: string;
	weekdayWednesday: string;
	weekdayThursday: string;
	weekdayFriday: string;
	weekdaySaturday: string;
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
			tooltipLabel: dailyTooltipLabel(dailyPoint.date, labels),
			value: dailyPoint.value
		}));
	}
	if (mode === 'week') {
		return buildWeeklySeries(month, valueByDate, labels.weekLabelTemplate);
	}
	return buildMonthlySeries(month, dailyValues, labels.monthLabelTemplate);
}

function buildWeeklySeries(
	month: string,
	valueByDate: Map<string, number>,
	weekLabelTemplate: string
): ChartPoint[] {
	return calendarWeekRanges(month).map((weekRange, index) => {
		const value = eachDateInRange(weekRange.startDate, weekRange.endDate).reduce(
			(sum, date) => sum + (valueByDate.get(date) ?? 0),
			0
		);
		return {
			key: `${weekRange.startDate}/${weekRange.endDate}`,
			label: weekLabelTemplate.replace('{index}', String(index + 1)),
			tooltipLabel: weekLabelTemplate.replace('{index}', String(index + 1)),
			value
		};
	});
}

function calendarWeekRanges(month: string): { startDate: string; endDate: string }[] {
	const monthDays = eachDayOfMonth(month);
	if (!monthDays.length) return [];
	const firstWeekStartDate = sundayWeekStart(monthDays[0]);
	const lastWeekEndDate = addDays(sundayWeekStart(monthDays[monthDays.length - 1]), 6);
	const weekRanges: { startDate: string; endDate: string }[] = [];
	for (
		let startDate = firstWeekStartDate;
		startDate <= lastWeekEndDate;
		startDate = addDays(startDate, 7)
	) {
		weekRanges.push({ startDate, endDate: addDays(startDate, 6) });
	}
	return weekRanges;
}

function sundayWeekStart(date: string): string {
	const parsedDate = new Date(`${date}T00:00:00Z`);
	if (Number.isNaN(parsedDate.getTime())) return date;
	parsedDate.setUTCDate(parsedDate.getUTCDate() - parsedDate.getUTCDay());
	return utcDateKey(parsedDate);
}

function eachDateInRange(startDate: string, endDate: string): string[] {
	const dates: string[] = [];
	for (let date = startDate; date <= endDate; date = addDays(date, 1)) {
		dates.push(date);
	}
	return dates;
}

function buildMonthlySeries(
	month: string,
	dailyValues: DailyValue[],
	monthLabelTemplate: string
): ChartPoint[] {
	const year = month.slice(0, 4);
	return Array.from({ length: 12 }, (_, index) => {
		const monthIndex = index + 1;
		const monthKey = `${year}-${String(monthIndex).padStart(2, '0')}`;
		const label = monthLabelTemplate.replace('{index}', String(monthIndex));
		return {
			key: monthKey,
			label,
			tooltipLabel: label,
			value: dailyValues.reduce((sum, dailyValue) => {
				if (!dailyValue.date.startsWith(monthKey)) return sum;
				return sum + dailyValue.value;
			}, 0)
		};
	});
}

function dailyTooltipLabel(date: string, labels: WorkTimeChartLabels): string {
	const [, month, day] = date.split('-');
	return `${Number(month)}/${Number(day)} ${weekdayLabel(date, labels)}`;
}

function weekdayLabel(date: string, labels: WorkTimeChartLabels): string {
	const day = new Date(`${date}T00:00:00Z`).getUTCDay();
	const weekdays = [
		labels.weekdaySunday,
		labels.weekdayMonday,
		labels.weekdayTuesday,
		labels.weekdayWednesday,
		labels.weekdayThursday,
		labels.weekdayFriday,
		labels.weekdaySaturday,
	];
	return weekdays[day] ?? '';
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

import type { ChartMode } from '../attendance-context.svelte';
import { addDays, eachDayOfMonth, utcDateKey } from './attendance-date';

export type DailyValue = { date: string; minutesByLocation: Record<string, number> };
export type ChartPoint = { key: string; label: string; tooltipLabel: string; values: Record<string, number> };
export type WorkTimeChartLocation = { key: string; name: string; color?: string };

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

export type BuildSeriesOptions = { today?: string };

export function chartPointTotalMinutes(point: ChartPoint): number {
	return Object.values(point.values).reduce((sum, minutes) => sum + minutes, 0);
}

export function buildSeries(
	month: string,
	dailyValues: DailyValue[],
	mode: ChartMode,
	labels: WorkTimeChartLabels,
	options: BuildSeriesOptions = {}
): ChartPoint[] {
	if (!month) return [];
	const valuesByDate = new Map(dailyValues.map((dailyValue) => [dailyValue.date, dailyValue.minutesByLocation]));
	const dailyPoints = eachDayOfMonth(month).map((date) => ({ date, values: valuesByDate.get(date) ?? {} }));

	if (mode === 'day') {
		const visibleDailyPoints = trimFuturePoints(dailyPoints, options.today, (dailyPoint) => dailyPoint.date);
		return visibleDailyPoints.map((dailyPoint) => ({
			key: dailyPoint.date,
			label: dailyPoint.date.slice(8, 10),
			tooltipLabel: dailyTooltipLabel(dailyPoint.date, labels),
			values: dailyPoint.values
		}));
	}
	if (mode === 'week') {
		return buildWeeklySeries(month, valuesByDate, labels.weekLabelTemplate, options.today);
	}
	return buildMonthlySeries(month, dailyValues, labels.monthLabelTemplate, options.today);
}

function sumMinutesByLocation(valuesList: Record<string, number>[]): Record<string, number> {
	const totals: Record<string, number> = {};
	for (const values of valuesList) {
		for (const [location, minutes] of Object.entries(values)) {
			totals[location] = (totals[location] ?? 0) + minutes;
		}
	}
	return totals;
}

function trimFuturePoints<T>(points: T[], today: string | undefined, dateOf: (point: T) => string): T[] {
	if (!today) return points;
	const pastOrPresentPoints = points.filter((point) => dateOf(point) <= today);
	if (!pastOrPresentPoints.length && points.length) return points;
	return pastOrPresentPoints;
}

function buildWeeklySeries(
	month: string,
	valuesByDate: Map<string, Record<string, number>>,
	weekLabelTemplate: string,
	today?: string
): ChartPoint[] {
	const weekRanges = trimFuturePoints(calendarWeekRanges(month), today, (weekRange) => weekRange.startDate);
	return weekRanges.map((weekRange, index) => {
		const values = sumMinutesByLocation(
			eachDateInRange(weekRange.startDate, weekRange.endDate).map((date) => valuesByDate.get(date) ?? {})
		);
		return {
			key: `${weekRange.startDate}/${weekRange.endDate}`,
			label: weekLabelTemplate.replace('{index}', String(index + 1)),
			tooltipLabel: weekLabelTemplate.replace('{index}', String(index + 1)),
			values
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
	monthLabelTemplate: string,
	today?: string
): ChartPoint[] {
	const year = month.slice(0, 4);
	const monthlyPoints = Array.from({ length: 12 }, (_, index) => {
		const monthIndex = index + 1;
		const monthKey = `${year}-${String(monthIndex).padStart(2, '0')}`;
		const label = monthLabelTemplate.replace('{index}', String(monthIndex));
		return {
			key: monthKey,
			label,
			tooltipLabel: label,
			values: sumMinutesByLocation(
				dailyValues
					.filter((dailyValue) => dailyValue.date.startsWith(monthKey))
					.map((dailyValue) => dailyValue.minutesByLocation)
			)
		};
	});
	if (!today || year !== today.slice(0, 4)) return monthlyPoints;
	const todayMonthKey = today.slice(0, 7);
	return monthlyPoints.filter((monthlyPoint) => monthlyPoint.key <= todayMonthKey);
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

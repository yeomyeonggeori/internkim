import { describe, expect, test } from 'bun:test';
import {
	buildTaskLineChartData,
	buildTaskLineGridTicks,
	buildTaskLineXAxisTicks,
	buildTaskLineYAxisMax,
	buildTaskLineYAxisTicks,
	defaultTaskLineChartLayout,
	taskLinePointsForSeries,
	taskLineShouldAlignTooltipEnd,
	taskLineTooltipPositionPercent,
	taskLineXForIndex,
	taskLineYForValue
} from '../../src/routes/task/report/task-line-chart-geometry';

describe('flow line chart geometry', () => {
	test('builds aligned chart data from labels and series values', () => {
		const data = buildTaskLineChartData({
			labels: ['월', '화'],
			currentValues: [5, 8, 13],
			previousValues: [3]
		});

		expect(data).toEqual([
			{ index: 0, label: '월', current: 5, previous: 3 },
			{ index: 1, label: '화', current: 8, previous: 0 },
			{ index: 2, label: '3', current: 13, previous: 0 }
		]);
	});

	test('keeps weekly x-axis labels aligned with plotted point positions', () => {
		expect(taskLineXForIndex(0, 7)).toBe(defaultTaskLineChartLayout.plotLeft);
		expect(taskLineXForIndex(6, 7)).toBe(defaultTaskLineChartLayout.chartWidth - defaultTaskLineChartLayout.plotRight);
		expect(taskLineXForIndex(3, 7)).toBe(372);
		expect(buildTaskLineXAxisTicks(7, false, 390)).toEqual([0, 1, 2, 3, 4, 5, 6]);
	});

	test('reduces monthly x-axis ticks on narrow viewports', () => {
		expect(buildTaskLineXAxisTicks(30, true, 720).length).toBe(30);
		expect(buildTaskLineXAxisTicks(30, true, 390)).toEqual([0, 5, 10, 15, 20, 25, 29]);
	});

	test('builds readable y-axis and grid ticks from data values', () => {
		const data = buildTaskLineChartData({
			labels: ['1', '2', '3'],
			currentValues: [5, 16, 21],
			previousValues: [1, 10, 17]
		});
		const maxValue = buildTaskLineYAxisMax(data);

		expect(maxValue).toBe(25);
		expect(buildTaskLineYAxisTicks(maxValue)).toEqual([0, 5, 10, 15, 20, 25]);
		expect(buildTaskLineGridTicks(maxValue)).toEqual([0, 2.5, 5, 7.5, 10, 12.5, 15, 17.5, 20, 22.5, 25]);
		expect(taskLineYForValue(0, maxValue)).toBe(278);
		expect(taskLineYForValue(25, maxValue)).toBe(12);
	});

	test('positions tooltip beside the active point and flips near the right edge', () => {
		const datum = { index: 28, label: '29', current: 21, previous: 17 };
		const position = taskLineTooltipPositionPercent(28, datum, 30, 25);

		expect(Math.round(position.x)).toBe(94);
		expect(Math.round(position.y)).toBe(17);
		expect(taskLineShouldAlignTooltipEnd(27, 30)).toBe(true);
		expect(taskLineShouldAlignTooltipEnd(26, 30)).toBe(false);
	});

	test('flips monthly tooltip before it overflows the visible chart width', () => {
		expect(taskLineShouldAlignTooltipEnd(22, 30, 444, 160)).toBe(true);
		expect(taskLineShouldAlignTooltipEnd(17, 30, 444, 160)).toBe(false);
	});

	test('returns polyline points from the same geometry as individual points', () => {
		const data = buildTaskLineChartData({
			labels: ['월', '화'],
			currentValues: [5, 10],
			previousValues: [4, 8]
		});

		expect(taskLinePointsForSeries(data, 'current', 10)).toBe('42,145 702,12');
	});
});

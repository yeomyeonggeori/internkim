export type TaskLineChartTrend = {
	labels: string[];
	currentValues: number[];
	previousValues: number[];
};

export type TaskLineChartDatum = {
	index: number;
	label: string;
	current: number;
	previous: number;
};

export type TaskLineChartLayout = {
	chartWidth: number;
	chartHeight: number;
	plotLeft: number;
	plotRight: number;
	plotTop: number;
	plotBottom: number;
};

export type TaskLineChartPoint = {
	x: number;
	y: number;
};

export const defaultTaskLineChartLayout: TaskLineChartLayout = {
	chartWidth: 720,
	chartHeight: 320,
	plotLeft: 42,
	plotRight: 18,
	plotTop: 12,
	plotBottom: 42
};

export function buildTaskLineChartData(trend: TaskLineChartTrend): TaskLineChartDatum[] {
	const pointCount = Math.max(trend.labels.length, trend.currentValues.length, trend.previousValues.length);
	return Array.from({ length: pointCount }, (_, index) => ({
		index,
		label: trend.labels[index] ?? String(index + 1),
		current: trend.currentValues[index] ?? 0,
		previous: trend.previousValues[index] ?? 0
	}));
}

export function buildTaskLineXAxisTicks(pointCount: number, isMonthlyChart: boolean, viewportWidth: number, chartWidth = defaultTaskLineChartLayout.chartWidth): number[] {
	if (!isMonthlyChart || pointCount <= 7) return Array.from({ length: pointCount }, (_, index) => index);
	if (canShowEveryMonthlyTick(pointCount, viewportWidth, chartWidth)) return Array.from({ length: pointCount }, (_, index) => index);

	const approximateTickCount = Math.max(2, Math.floor((viewportWidth || chartWidth) / 44));
	const step = Math.max(1, Math.ceil((pointCount - 1) / Math.max(1, approximateTickCount - 1)));
	const ticks = Array.from({ length: pointCount }, (_, index) => index).filter((index) => index === 0 || index === pointCount - 1 || index % step === 0);
	return Array.from(new Set(ticks));
}

export function canShowEveryMonthlyTick(pointCount: number, viewportWidth: number, chartWidth = defaultTaskLineChartLayout.chartWidth): boolean {
	const availableWidth = viewportWidth || chartWidth;
	return availableWidth / Math.max(1, pointCount) >= 18;
}

export function buildTaskLineYAxisMax(data: TaskLineChartDatum[]): number {
	const maxValue = Math.max(1, ...data.flatMap((datum) => [datum.current, datum.previous]));
	const step = niceTaskLineStep(maxValue / 5);
	return Math.ceil(maxValue / step) * step;
}

export function buildTaskLineYAxisTicks(maxValue: number): number[] {
	const step = niceTaskLineStep(maxValue / 5);
	return taskLineTicksForStep(maxValue, step);
}

export function buildTaskLineGridTicks(maxValue: number): number[] {
	const majorStep = niceTaskLineStep(maxValue / 5);
	return taskLineTicksForStep(maxValue, majorStep / 2);
}

export function taskLineTicksForStep(maxValue: number, step: number): number[] {
	const count = Math.floor(maxValue / step);
	return Array.from({ length: count + 1 }, (_, index) => roundTaskLineTick(index * step));
}

export function niceTaskLineStep(value: number): number {
	if (value <= 0) return 1;
	const magnitude = 10 ** Math.floor(Math.log10(value));
	const normalizedValue = value / magnitude;
	if (normalizedValue <= 1) return magnitude;
	if (normalizedValue <= 2) return 2 * magnitude;
	if (normalizedValue <= 5) return 5 * magnitude;
	return 10 * magnitude;
}

export function roundTaskLineTick(value: number): number {
	return Math.round(value * 100) / 100;
}

export function taskLinePointsForSeries(data: TaskLineChartDatum[], key: 'current' | 'previous', maxValue: number, layout = defaultTaskLineChartLayout): string {
	return data.map((datum) => `${taskLineXForIndex(datum.index, data.length, layout)},${taskLineYForValue(datum[key], maxValue, layout)}`).join(' ');
}

export function taskLineXForIndex(index: number, pointCount: number, layout = defaultTaskLineChartLayout): number {
	const plotWidth = taskLinePlotWidth(layout);
	if (pointCount <= 1) return layout.plotLeft + plotWidth / 2;
	return layout.plotLeft + (index / (pointCount - 1)) * plotWidth;
}

export function taskLineYForValue(value: number, maxValue: number, layout = defaultTaskLineChartLayout): number {
	if (maxValue <= 0) return layout.plotTop + taskLinePlotHeight(layout);
	return layout.plotTop + taskLinePlotHeight(layout) - (value / maxValue) * taskLinePlotHeight(layout);
}

export function taskLineTooltipPositionPercent(index: number, datum: TaskLineChartDatum, pointCount: number, maxValue: number, layout = defaultTaskLineChartLayout): TaskLineChartPoint {
	const x = taskLineXForIndex(index, pointCount, layout);
	const y = taskLineYForValue(Math.max(datum.current, datum.previous), maxValue, layout);
	return {
		x: (x / layout.chartWidth) * 100,
		y: (y / layout.chartHeight) * 100
	};
}

export function taskLineShouldAlignTooltipEnd(index: number, pointCount: number, visibleChartWidth = 0, tooltipWidth = 0, tooltipGap = 12, layout = defaultTaskLineChartLayout): boolean {
	if (visibleChartWidth > 0 && tooltipWidth > 0) {
		const x = taskLineXForIndex(index, pointCount, layout);
		const visibleChartX = (x / layout.chartWidth) * visibleChartWidth;
		return visibleChartX + tooltipGap + tooltipWidth > visibleChartWidth;
	}
	return index >= pointCount - 3;
}

export function taskLinePlotWidth(layout = defaultTaskLineChartLayout): number {
	return layout.chartWidth - layout.plotLeft - layout.plotRight;
}

export function taskLinePlotHeight(layout = defaultTaskLineChartLayout): number {
	return layout.chartHeight - layout.plotTop - layout.plotBottom;
}

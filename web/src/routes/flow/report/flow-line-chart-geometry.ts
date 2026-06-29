export type FlowLineChartTrend = {
	labels: string[];
	currentValues: number[];
	previousValues: number[];
};

export type FlowLineChartDatum = {
	index: number;
	label: string;
	current: number;
	previous: number;
};

export type FlowLineChartLayout = {
	chartWidth: number;
	chartHeight: number;
	plotLeft: number;
	plotRight: number;
	plotTop: number;
	plotBottom: number;
};

export type FlowLineChartPoint = {
	x: number;
	y: number;
};

export const defaultFlowLineChartLayout: FlowLineChartLayout = {
	chartWidth: 720,
	chartHeight: 320,
	plotLeft: 42,
	plotRight: 18,
	plotTop: 12,
	plotBottom: 42
};

export function buildFlowLineChartData(trend: FlowLineChartTrend): FlowLineChartDatum[] {
	const pointCount = Math.max(trend.labels.length, trend.currentValues.length, trend.previousValues.length);
	return Array.from({ length: pointCount }, (_, index) => ({
		index,
		label: trend.labels[index] ?? String(index + 1),
		current: trend.currentValues[index] ?? 0,
		previous: trend.previousValues[index] ?? 0
	}));
}

export function buildFlowLineXAxisTicks(pointCount: number, isMonthlyChart: boolean, viewportWidth: number, chartWidth = defaultFlowLineChartLayout.chartWidth): number[] {
	if (!isMonthlyChart || pointCount <= 7) return Array.from({ length: pointCount }, (_, index) => index);
	if (canShowEveryMonthlyTick(pointCount, viewportWidth, chartWidth)) return Array.from({ length: pointCount }, (_, index) => index);

	const approximateTickCount = Math.max(2, Math.floor((viewportWidth || chartWidth) / 44));
	const step = Math.max(1, Math.ceil((pointCount - 1) / Math.max(1, approximateTickCount - 1)));
	const ticks = Array.from({ length: pointCount }, (_, index) => index).filter((index) => index === 0 || index === pointCount - 1 || index % step === 0);
	return Array.from(new Set(ticks));
}

export function canShowEveryMonthlyTick(pointCount: number, viewportWidth: number, chartWidth = defaultFlowLineChartLayout.chartWidth): boolean {
	const availableWidth = viewportWidth || chartWidth;
	return availableWidth / Math.max(1, pointCount) >= 18;
}

export function buildFlowLineYAxisMax(data: FlowLineChartDatum[]): number {
	const maxValue = Math.max(1, ...data.flatMap((datum) => [datum.current, datum.previous]));
	const step = niceFlowLineStep(maxValue / 5);
	return Math.ceil(maxValue / step) * step;
}

export function buildFlowLineYAxisTicks(maxValue: number): number[] {
	const step = niceFlowLineStep(maxValue / 5);
	return flowLineTicksForStep(maxValue, step);
}

export function buildFlowLineGridTicks(maxValue: number): number[] {
	const majorStep = niceFlowLineStep(maxValue / 5);
	return flowLineTicksForStep(maxValue, majorStep / 2);
}

export function flowLineTicksForStep(maxValue: number, step: number): number[] {
	const count = Math.floor(maxValue / step);
	return Array.from({ length: count + 1 }, (_, index) => roundFlowLineTick(index * step));
}

export function niceFlowLineStep(value: number): number {
	if (value <= 0) return 1;
	const magnitude = 10 ** Math.floor(Math.log10(value));
	const normalizedValue = value / magnitude;
	if (normalizedValue <= 1) return magnitude;
	if (normalizedValue <= 2) return 2 * magnitude;
	if (normalizedValue <= 5) return 5 * magnitude;
	return 10 * magnitude;
}

export function roundFlowLineTick(value: number): number {
	return Math.round(value * 100) / 100;
}

export function flowLinePointsForSeries(data: FlowLineChartDatum[], key: 'current' | 'previous', maxValue: number, layout = defaultFlowLineChartLayout): string {
	return data.map((datum) => `${flowLineXForIndex(datum.index, data.length, layout)},${flowLineYForValue(datum[key], maxValue, layout)}`).join(' ');
}

export function flowLineXForIndex(index: number, pointCount: number, layout = defaultFlowLineChartLayout): number {
	const plotWidth = flowLinePlotWidth(layout);
	if (pointCount <= 1) return layout.plotLeft + plotWidth / 2;
	return layout.plotLeft + (index / (pointCount - 1)) * plotWidth;
}

export function flowLineYForValue(value: number, maxValue: number, layout = defaultFlowLineChartLayout): number {
	if (maxValue <= 0) return layout.plotTop + flowLinePlotHeight(layout);
	return layout.plotTop + flowLinePlotHeight(layout) - (value / maxValue) * flowLinePlotHeight(layout);
}

export function flowLineTooltipPositionPercent(index: number, datum: FlowLineChartDatum, pointCount: number, maxValue: number, layout = defaultFlowLineChartLayout): FlowLineChartPoint {
	const x = flowLineXForIndex(index, pointCount, layout);
	const y = flowLineYForValue(Math.max(datum.current, datum.previous), maxValue, layout);
	return {
		x: (x / layout.chartWidth) * 100,
		y: (y / layout.chartHeight) * 100
	};
}

export function flowLineShouldAlignTooltipEnd(index: number, pointCount: number, visibleChartWidth = 0, tooltipWidth = 0, tooltipGap = 12, layout = defaultFlowLineChartLayout): boolean {
	if (visibleChartWidth > 0 && tooltipWidth > 0) {
		const x = flowLineXForIndex(index, pointCount, layout);
		const visibleChartX = (x / layout.chartWidth) * visibleChartWidth;
		return visibleChartX + tooltipGap + tooltipWidth > visibleChartWidth;
	}
	return index >= pointCount - 3;
}

export function flowLinePlotWidth(layout = defaultFlowLineChartLayout): number {
	return layout.chartWidth - layout.plotLeft - layout.plotRight;
}

export function flowLinePlotHeight(layout = defaultFlowLineChartLayout): number {
	return layout.chartHeight - layout.plotTop - layout.plotBottom;
}

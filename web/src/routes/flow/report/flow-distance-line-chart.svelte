<script lang="ts">
	import type { FlowTrendSection } from './flow-report-data';
	import {
		buildFlowLineChartData,
		buildFlowLineGridTicks,
		buildFlowLineXAxisTicks,
		buildFlowLineYAxisMax,
		buildFlowLineYAxisTicks,
		defaultFlowLineChartLayout,
		flowLinePointsForSeries,
		flowLinePlotHeight,
		flowLinePlotWidth,
		flowLineShouldAlignTooltipEnd,
		flowLineTooltipPositionPercent,
		flowLineXForIndex,
		flowLineYForValue,
		type FlowLineChartDatum
	} from './flow-line-chart-geometry';

	type LineChartVariant = 'weekly' | 'monthly';

	type Props = {
		section: FlowTrendSection;
		variant: LineChartVariant;
	};

	let { section, variant }: Props = $props();
	let activeIndex = $state<number | null>(null);
	let chartViewportWidth = $state(0);

	const chartWidth = defaultFlowLineChartLayout.chartWidth;
	const chartHeight = defaultFlowLineChartLayout.chartHeight;
	const plotLeft = defaultFlowLineChartLayout.plotLeft;
	const plotRight = defaultFlowLineChartLayout.plotRight;
	const plotTop = defaultFlowLineChartLayout.plotTop;
	const currentColor = '#0f766e';
	const previousColor = '#94a3b8';
	const tooltipWidth = 160;
	const tooltipGap = 12;

	const chartData = $derived(buildFlowLineChartData(section.trend));
	const isMonthlyChart = $derived(variant === 'monthly');
	const yAxisMax = $derived(buildFlowLineYAxisMax(chartData));
	const yAxisTicks = $derived(buildFlowLineYAxisTicks(yAxisMax));
	const yGridTicks = $derived(buildFlowLineGridTicks(yAxisMax));
	const xAxisTicks = $derived(buildFlowLineXAxisTicks(chartData.length, isMonthlyChart, chartViewportWidth));
	const currentPoints = $derived(flowLinePointsForSeries(chartData, 'current', yAxisMax));
	const previousPoints = $derived(flowLinePointsForSeries(chartData, 'previous', yAxisMax));
	const activeDatum = $derived(activeIndex === null ? null : (chartData[activeIndex] ?? null));
	const plotWidth = flowLinePlotWidth();
	const plotHeight = flowLinePlotHeight();

	function formatValue(value: number): string {
		return `${value}${section.unit}`;
	}

	function formatYAxisValue(value: number): string {
		return `${Math.round(value)}${section.unit}`;
	}

	function formatDelta(value: number): string {
		if (value > 0) return `+${formatValue(value)}`;
		return formatValue(value);
	}

	function deltaClass(value: number): string {
		if (value > 0) return 'text-teal-700';
		if (value < 0) return 'text-rose-700';
		return 'text-muted-foreground';
	}

	function tooltipLabel(datum: FlowLineChartDatum): string {
		if (section.trend.labelTemplate) return section.trend.labelTemplate.replace('{day}', datum.label);
		return datum.label;
	}

	function tooltipStyle(index: number, datum: FlowLineChartDatum): string {
		const position = flowLineTooltipPositionPercent(index, datum, chartData.length, yAxisMax);
		return `left: ${position.x}%; top: ${position.y}%`;
	}

	function tooltipClass(index: number): string {
		const edgeClass = flowLineShouldAlignTooltipEnd(index, chartData.length, chartViewportWidth, tooltipWidth, tooltipGap) ? '-translate-x-full -ml-3' : 'ml-3';
		return `pointer-events-none absolute z-20 min-w-40 -translate-y-1/2 rounded-md border bg-card/95 px-3 py-2 text-xs shadow-sm backdrop-blur ${edgeClass}`;
	}

	function handleDatumKeydown(event: KeyboardEvent, index: number): void {
		if (event.key === 'Escape') {
			activeIndex = null;
			return;
		}
		if (event.key !== 'Enter' && event.key !== ' ') return;

		event.preventDefault();
		activeIndex = activeIndex === index ? null : index;
	}
</script>

<div class="space-y-4">
	<div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
		<div class="flex items-center gap-1.5">
			<span class="h-0.5 w-4 rounded-full bg-teal-700"></span>
			<span>{section.trend.currentLabel}</span>
			<span class="font-medium text-foreground tabular-nums">{formatValue(section.trend.currentTotal)}</span>
		</div>
		<div class="flex items-center gap-1.5">
			<span class="h-0.5 w-4 rounded-full bg-slate-400"></span>
			<span>{section.trend.previousLabel}</span>
			<span class="font-medium text-foreground tabular-nums">{formatValue(section.trend.previousTotal)}</span>
		</div>
		<div class={`font-medium tabular-nums ${deltaClass(section.alertValue)}`}>{formatDelta(section.alertValue)}</div>
	</div>

	<div bind:offsetWidth={chartViewportWidth} class="relative h-[22rem] min-h-[22rem] w-full overflow-visible rounded-md border bg-card px-3 py-2">
		<svg viewBox={`0 0 ${chartWidth} ${chartHeight}`} class="h-full w-full overflow-visible" role="img" aria-label={section.title}>
			{#each yGridTicks as tick}
				<line
					x1={plotLeft}
					x2={chartWidth - plotRight}
					y1={flowLineYForValue(tick, yAxisMax)}
					y2={flowLineYForValue(tick, yAxisMax)}
					class={yAxisTicks.includes(tick) ? 'stroke-border/70' : 'stroke-border/35'}
					stroke-width="1"
				/>
			{/each}

			{#each yAxisTicks as tick}
				<text x={plotLeft - 8} y={flowLineYForValue(tick, yAxisMax) + 4} text-anchor="end" class="fill-muted-foreground text-[11px]">{formatYAxisValue(tick)}</text>
			{/each}

			<polyline points={previousPoints} fill="none" stroke={previousColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
			<polyline points={currentPoints} fill="none" stroke={currentColor} stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />

			{#each chartData as datum}
				<circle cx={flowLineXForIndex(datum.index, chartData.length)} cy={flowLineYForValue(datum.previous, yAxisMax)} r={isMonthlyChart ? 2.5 : 3.5} fill={previousColor} stroke="white" stroke-width="1.5" />
				<circle cx={flowLineXForIndex(datum.index, chartData.length)} cy={flowLineYForValue(datum.current, yAxisMax)} r={isMonthlyChart ? 2.5 : 3.5} fill={currentColor} stroke="white" stroke-width="1.5" />
			{/each}

			{#each xAxisTicks as tickIndex}
				{@const datum = chartData[tickIndex]}
				{#if datum}
					<text
						x={flowLineXForIndex(tickIndex, chartData.length)}
						y={chartHeight - 10}
						text-anchor="middle"
						class={isMonthlyChart ? 'fill-muted-foreground text-[9px]' : 'fill-muted-foreground text-[11px]'}
					>
						{datum.label}
					</text>
				{/if}
			{/each}

			{#each chartData as datum, index}
				<rect
					x={flowLineXForIndex(index, chartData.length) - plotWidth / Math.max(2, chartData.length - 1) / 2}
					y={plotTop}
					width={plotWidth / Math.max(1, chartData.length - 1)}
					height={plotHeight}
					fill="transparent"
					role="button"
					tabindex="0"
					aria-label={`${tooltipLabel(datum)} ${section.trend.currentLabel} ${formatValue(datum.current)}, ${section.trend.previousLabel} ${formatValue(datum.previous)}`}
					onmouseenter={() => (activeIndex = index)}
					onmouseleave={() => (activeIndex = null)}
					onfocus={() => (activeIndex = index)}
					onblur={() => (activeIndex = null)}
					onkeydown={(event) => handleDatumKeydown(event, index)}
				/>
			{/each}
		</svg>

		{#if activeDatum && activeIndex !== null}
			<div class={tooltipClass(activeIndex)} style={tooltipStyle(activeIndex, activeDatum)}>
				<div class="mb-1 font-medium text-foreground">{tooltipLabel(activeDatum)}</div>
				<div class="grid gap-1 text-muted-foreground">
					<div class="flex items-center justify-between gap-6">
						<span class="flex items-center gap-1.5">
							<span class="h-0.5 w-3 rounded-full bg-teal-700"></span>
							{section.trend.currentLabel}
						</span>
						<span class="font-medium text-foreground tabular-nums">{formatValue(activeDatum.current)}</span>
					</div>
					<div class="flex items-center justify-between gap-6">
						<span class="flex items-center gap-1.5">
							<span class="h-0.5 w-3 rounded-full bg-slate-400"></span>
							{section.trend.previousLabel}
						</span>
						<span class="font-medium text-foreground tabular-nums">{formatValue(activeDatum.previous)}</span>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

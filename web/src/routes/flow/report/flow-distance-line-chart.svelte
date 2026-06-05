<script lang="ts">
	import type { FlowReportSection, FlowReportTrend } from './flow-report-data';

	type LineChartVariant = 'weekly' | 'monthly';

	type Props = {
		section: FlowReportSection;
		variant: LineChartVariant;
	};

	type DistanceTrendDatum = {
		index: number;
		label: string;
		current: number;
		previous: number;
	};

	let { section, variant }: Props = $props();
	let activeIndex = $state<number | null>(null);
	let chartViewportWidth = $state(0);

	const chartWidth = 720;
	const chartHeight = 320;
	const plotLeft = 42;
	const plotRight = 18;
	const plotTop = 12;
	const plotBottom = 42;
	const currentColor = '#0f766e';
	const previousColor = '#94a3b8';

	const chartData = $derived(buildChartData(section.trend));
	const isMonthlyChart = $derived(variant === 'monthly');
	const yAxisMax = $derived(buildYAxisMax(chartData));
	const yAxisTicks = $derived(buildYAxisTicks(yAxisMax));
	const yGridTicks = $derived(buildYGridTicks(yAxisMax));
	const xAxisTicks = $derived(buildXAxisTicks(chartData.length, isMonthlyChart, chartViewportWidth));
	const currentPoints = $derived(pointsForSeries(chartData, 'current', yAxisMax));
	const previousPoints = $derived(pointsForSeries(chartData, 'previous', yAxisMax));
	const activeDatum = $derived(activeIndex === null ? null : (chartData[activeIndex] ?? null));
	const plotWidth = chartWidth - plotLeft - plotRight;
	const plotHeight = chartHeight - plotTop - plotBottom;

	function buildChartData(trend: FlowReportTrend): DistanceTrendDatum[] {
		const pointCount = Math.max(trend.labels.length, trend.currentValues.length, trend.previousValues.length);
		return Array.from({ length: pointCount }, (_, index) => ({
			index,
			label: trend.labels[index] ?? String(index + 1),
			current: trend.currentValues[index] ?? 0,
			previous: trend.previousValues[index] ?? 0
		}));
	}

	function buildXAxisTicks(pointCount: number, shouldShowMonthlyTicks: boolean, viewportWidth: number): number[] {
		if (!shouldShowMonthlyTicks || pointCount <= 7) return Array.from({ length: pointCount }, (_, index) => index);
		if (canShowEveryMonthlyTick(pointCount, viewportWidth)) return Array.from({ length: pointCount }, (_, index) => index);

		const approximateTickCount = Math.max(2, Math.floor((viewportWidth || chartWidth) / 44));
		const step = Math.max(1, Math.ceil((pointCount - 1) / Math.max(1, approximateTickCount - 1)));
		const ticks = Array.from({ length: pointCount }, (_, index) => index).filter((index) => index === 0 || index === pointCount - 1 || index % step === 0);
		return Array.from(new Set(ticks));
	}

	function canShowEveryMonthlyTick(pointCount: number, viewportWidth: number): boolean {
		const availableWidth = viewportWidth || chartWidth;
		return availableWidth / Math.max(1, pointCount) >= 18;
	}

	function buildYAxisMax(data: DistanceTrendDatum[]): number {
		const maxValue = Math.max(1, ...data.flatMap((datum) => [datum.current, datum.previous]));
		const step = niceStep(maxValue / 5);
		return Math.ceil(maxValue / step) * step;
	}

	function buildYAxisTicks(maxValue: number): number[] {
		const step = niceStep(maxValue / 5);
		return ticksForStep(maxValue, step);
	}

	function buildYGridTicks(maxValue: number): number[] {
		const majorStep = niceStep(maxValue / 5);
		return ticksForStep(maxValue, majorStep / 2);
	}

	function ticksForStep(maxValue: number, step: number): number[] {
		const count = Math.floor(maxValue / step);
		return Array.from({ length: count + 1 }, (_, index) => roundTick(index * step));
	}

	function niceStep(value: number): number {
		if (value <= 0) return 1;
		const magnitude = 10 ** Math.floor(Math.log10(value));
		const normalizedValue = value / magnitude;
		if (normalizedValue <= 1) return magnitude;
		if (normalizedValue <= 2) return 2 * magnitude;
		if (normalizedValue <= 5) return 5 * magnitude;
		return 10 * magnitude;
	}

	function roundTick(value: number): number {
		return Math.round(value * 100) / 100;
	}

	function pointsForSeries(data: DistanceTrendDatum[], key: 'current' | 'previous', maxValue: number): string {
		return data.map((datum) => `${xForIndex(datum.index, data.length)},${yForValue(datum[key], maxValue)}`).join(' ');
	}

	function xForIndex(index: number, pointCount: number): number {
		if (pointCount <= 1) return plotLeft + plotWidth / 2;
		return plotLeft + (index / (pointCount - 1)) * plotWidth;
	}

	function yForValue(value: number, maxValue: number): number {
		if (maxValue <= 0) return plotTop + plotHeight;
		return plotTop + plotHeight - (value / maxValue) * plotHeight;
	}

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

	function tooltipLabel(datum: DistanceTrendDatum): string {
		if (section.trend.labels.length > 7) return `${datum.label}일`;
		return datum.label;
	}

	function tooltipStyle(index: number, datum: DistanceTrendDatum): string {
		const x = xForIndex(index, chartData.length);
		const y = yForValue(Math.max(datum.current, datum.previous), yAxisMax);
		return `left: ${(x / chartWidth) * 100}%; top: ${(y / chartHeight) * 100}%`;
	}

	function tooltipClass(index: number): string {
		const edgeClass = index >= chartData.length - 3 ? '-translate-x-full -ml-3' : 'ml-3';
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
					y1={yForValue(tick, yAxisMax)}
					y2={yForValue(tick, yAxisMax)}
					class={yAxisTicks.includes(tick) ? 'stroke-border/70' : 'stroke-border/35'}
					stroke-width="1"
				/>
			{/each}

			{#each yAxisTicks as tick}
				<text x={plotLeft - 8} y={yForValue(tick, yAxisMax) + 4} text-anchor="end" class="fill-muted-foreground text-[11px]">{formatYAxisValue(tick)}</text>
			{/each}

			<polyline points={previousPoints} fill="none" stroke={previousColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
			<polyline points={currentPoints} fill="none" stroke={currentColor} stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />

			{#each chartData as datum}
				<circle cx={xForIndex(datum.index, chartData.length)} cy={yForValue(datum.previous, yAxisMax)} r={isMonthlyChart ? 2.5 : 3.5} fill={previousColor} stroke="white" stroke-width="1.5" />
				<circle cx={xForIndex(datum.index, chartData.length)} cy={yForValue(datum.current, yAxisMax)} r={isMonthlyChart ? 2.5 : 3.5} fill={currentColor} stroke="white" stroke-width="1.5" />
			{/each}

			{#each xAxisTicks as tickIndex}
				{@const datum = chartData[tickIndex]}
				{#if datum}
					<text
						x={xForIndex(tickIndex, chartData.length)}
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
					x={xForIndex(index, chartData.length) - plotWidth / Math.max(2, chartData.length - 1) / 2}
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

<script lang="ts">
	import * as Chart from '$lib/components/ui/chart';
	import { LineChart } from 'layerchart';
	import type { FlowTrendSection } from './flow-report-data';
	import { buildFlowLineChartData, buildFlowLineXAxisTicks } from './flow-line-chart-geometry';

	type LineChartVariant = 'weekly' | 'monthly';

	type Props = {
		section: FlowTrendSection;
		variant: LineChartVariant;
	};

	let { section, variant }: Props = $props();
	let chartViewportWidth = $state(0);

	const currentColor = 'var(--color-blue-600)';
	const previousColor = 'var(--color-slate-400)';

	const chartData = $derived(buildFlowLineChartData(section.trend));
	const isMonthlyChart = $derived(variant === 'monthly');
	const axisTickIndexes = $derived(buildFlowLineXAxisTicks(chartData.length, isMonthlyChart, chartViewportWidth));
	const chartConfig = $derived<Chart.ChartConfig>({
		current: { label: section.trend.currentLabel, color: currentColor },
		previous: { label: section.trend.previousLabel, color: previousColor }
	});
	const chartSeries = $derived([
		{ key: 'previous', label: section.trend.previousLabel, color: previousColor },
		{ key: 'current', label: section.trend.currentLabel, color: currentColor }
	]);

	function formatValue(value: number): string {
		return `${value}${section.unit}`;
	}

	function formatDelta(value: number): string {
		if (value > 0) return `+${formatValue(value)}`;
		return formatValue(value);
	}

	function deltaClass(value: number): string {
		if (value > 0) return 'text-blue-600';
		if (value < 0) return 'text-muted-foreground';
		return 'text-muted-foreground';
	}

	function formatAxisLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		if (!axisTickIndexes.includes(index)) return '';
		return chartData[index]?.label ?? '';
	}

	function formatTooltipLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		const label = chartData[index]?.label ?? String(value);
		if (section.trend.labelTemplate) return section.trend.labelTemplate.replace('{day}', label);
		return label;
	}

	function formatTooltipValue(value: unknown): string {
		if (typeof value !== 'number') return String(value);
		return formatValue(value);
	}
</script>

<div class="space-y-4">
	<div class="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
		<div class="flex items-center gap-1.5">
			<span class="h-0.5 w-4 rounded-full bg-blue-600"></span>
			<span>{section.trend.currentLabel}</span>
			<span class="text-foreground font-medium tabular-nums">{formatValue(section.trend.currentTotal)}</span>
		</div>
		<div class="flex items-center gap-1.5">
			<span class="h-0.5 w-4 rounded-full bg-slate-400"></span>
			<span>{section.trend.previousLabel}</span>
			<span class="text-foreground font-medium tabular-nums">{formatValue(section.trend.previousTotal)}</span>
		</div>
		<div class={`font-medium tabular-nums ${deltaClass(section.alertValue)}`}>{formatDelta(section.alertValue)}</div>
	</div>

	<div bind:offsetWidth={chartViewportWidth}>
		<Chart.Container config={chartConfig} class="bg-card h-[22rem] w-full rounded-md border px-3 py-2" aria-label={section.title}>
			<LineChart
				data={chartData}
				x="index"
				axis
				grid
				rule={false}
				series={chartSeries}
				padding={{ left: 32, right: 8, bottom: 20 }}
				props={{
					grid: { class: 'stroke-border/60' },
					xAxis: { ticks: axisTickIndexes, format: formatAxisLabel },
					yAxis: { format: formatValue, ticks: 5 },
					spline: { strokeWidth: 2.5 }
				}}
			>
				{#snippet tooltip()}
					<Chart.Tooltip
						anchor="bottom"
						contained={false}
						indicator="line"
						labelFormatter={formatTooltipLabel}
						class="w-max min-w-40"
						motion="none"
						x="data"
						y="data"
						yOffset={8}
					>
						{#snippet formatter({ value, name, item })}
							<div class="h-0.5 w-3 shrink-0 rounded-full" style={`background-color:${item.color}`}></div>
							<div class="flex flex-1 items-center justify-between gap-5 leading-none">
								<span class="text-muted-foreground whitespace-nowrap">{name}</span>
								<span class="text-foreground font-mono font-medium tabular-nums">{formatTooltipValue(value)}</span>
							</div>
						{/snippet}
					</Chart.Tooltip>
				{/snippet}
			</LineChart>
		</Chart.Container>
	</div>
</div>

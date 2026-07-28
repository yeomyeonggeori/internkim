<script lang="ts">
	import * as Chart from '$lib/components/ui/chart';
	import { BarChart } from 'layerchart';
	import type { ChartPoint } from './work-time-chart-model';

	type ChartSeries = {
		key: string;
		label: string;
		value: (point: ChartPoint) => number | undefined;
		color: string;
	};

	type Props = {
		title: string;
		compact: boolean;
		chartData: (ChartPoint & { index: number })[];
		chartConfig: Chart.ChartConfig;
		chartSeries: ChartSeries[];
		maxTotal: number;
		axisLabelIndexes: number[];
		axisTickLabelProps: { textAnchor: 'start' } | undefined;
		fallbackLocationColor: string;
		axisLabelFormatter: (value: unknown) => string;
		tooltipLabelFormatter: (value: unknown) => string;
		formatChartValue: (value: number) => string;
	};

	let {
		title,
		compact,
		chartData,
		chartConfig,
		chartSeries,
		maxTotal,
		axisLabelIndexes,
		axisTickLabelProps,
		fallbackLocationColor,
		axisLabelFormatter,
		tooltipLabelFormatter,
		formatChartValue
	}: Props = $props();
</script>

<Chart.Container
	config={chartConfig}
	class={compact ? 'h-24 w-full [&_.lc-axis-tick-label]:text-[9px]' : 'h-72 w-full'}
	aria-label={title}
>
	<BarChart
		data={chartData}
		x="index"
		yDomain={[0, maxTotal]}
		axis="x"
		grid
		rule={false}
		padding={{ left: 6, right: 6, bottom: 18 }}
		series={chartSeries}
		seriesLayout="stack"
		props={{
			grid: { class: 'stroke-border/60' },
			xAxis: {
				ticks: axisLabelIndexes,
				format: axisLabelFormatter,
				tickLabelProps: axisTickLabelProps
			},
			bars: {
				strokeWidth: 0,
				radius: 2
			}
		}}
	>
		{#snippet tooltip()}
			<Chart.Tooltip
				anchor="bottom"
				contained={false}
				indicator="dot"
				labelFormatter={tooltipLabelFormatter}
				class="w-max min-w-28"
				motion="none"
				x="data"
				y="data"
				yOffset={8}
			>
				{#snippet formatter({ value, name, item })}
					<div class="size-2.5 shrink-0 rounded-[2px]" style="background-color: {item.color ?? fallbackLocationColor};"></div>
					<div class="flex flex-1 items-center justify-between gap-3 leading-none">
						<span class="whitespace-nowrap text-muted-foreground">{name}</span>
						<span class="whitespace-nowrap font-mono font-medium tabular-nums text-foreground">
							{typeof value === 'number' ? formatChartValue(value) : String(value)}
						</span>
					</div>
				{/snippet}
			</Chart.Tooltip>
		{/snippet}
	</BarChart>
</Chart.Container>

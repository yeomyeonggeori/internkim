<script lang="ts">
	import * as Chart from '$lib/components/ui/chart';
	import { AreaChart } from 'layerchart';
	import {
		companyMetricDisplayValue,
		companyMetricPeriodLabel,
		formatCompanyMetricValue,
		type CompanyMetricDisplayCurrency,
		type CompanyShareMetric
	} from './company-page-model';

	let {
		metrics,
		label,
		displayCurrency,
		language,
		compact = false
	}: {
		metrics: CompanyShareMetric[];
		label: string;
		displayCurrency: CompanyMetricDisplayCurrency;
		language: 'ko' | 'en';
		compact?: boolean;
	} = $props();

	const chartData = $derived(metrics.map((metric, index) => ({ metric, index, value: companyMetricDisplayValue(metric, displayCurrency) })));
	const chartConfig = $derived<Chart.ChartConfig>({ value: { label, color: 'var(--color-blue-600)' } });
	const chartSeries = $derived([{ key: 'value', label, color: 'var(--color-blue-600)' }]);
	const tooltipLabels = $derived(new Map(chartData.map((point) => [point.index, companyMetricPeriodLabel(point.metric, language)])));
	const axisLabelIndexes = $derived(createAxisLabelIndexes(chartData.length, compact ? 2 : 4));

	function formatTooltipLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		return tooltipLabels.get(index) ?? String(value);
	}

	function formatAxisLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		if (!axisLabelIndexes.includes(index)) return '';
		return chartData[index] ? companyMetricPeriodLabel(chartData[index].metric, language) : '';
	}

	function formatTooltipValue(value: unknown): string {
		if (typeof value !== 'number' || metrics.length === 0) return String(value);
		const reference = metrics[metrics.length - 1];
		const metric = displayCurrency === 'USD' && reference.valueUSD !== undefined
			? { ...reference, valueUSD: value }
			: { ...reference, value };
		return formatCompanyMetricValue(metric, displayCurrency, language);
	}

	function createAxisLabelIndexes(count: number, maximumCount: number): number[] {
		if (count <= maximumCount) return Array.from({ length: count }, (_, index) => index);
		return Array.from({ length: maximumCount }, (_, index) => Math.round(index * (count - 1) / (maximumCount - 1)));
	}
</script>

<Chart.Container config={chartConfig} class={compact ? 'h-28 w-full' : 'h-56 w-full'} aria-label={label}>
	<AreaChart
		data={chartData}
		x="index"
		axis="x"
		grid
		rule={false}
		series={chartSeries}
		padding={{ left: 6, right: 6, bottom: 18 }}
		props={{
			grid: { class: 'stroke-border/60' },
			xAxis: { ticks: axisLabelIndexes, format: formatAxisLabel },
			area: { fillOpacity: 0.18, line: { strokeWidth: compact ? 2 : 3 } }
		}}
	>
		{#snippet tooltip()}
			<Chart.Tooltip
				anchor="bottom"
				contained={false}
				indicator="dot"
				labelFormatter={formatTooltipLabel}
				class="w-max min-w-36"
				motion="none"
				x="data"
				y="data"
				yOffset={8}
			>
				{#snippet formatter({ value, name, item })}
					<div class="size-2.5 shrink-0 rounded-[2px]" style={`background-color:${item.color}`}></div>
					<div class="flex flex-1 items-center justify-between gap-5 leading-none">
						<span class="text-muted-foreground whitespace-nowrap">{name}</span>
						<span class="text-foreground font-mono font-medium tabular-nums">{formatTooltipValue(value)}</span>
					</div>
				{/snippet}
			</Chart.Tooltip>
		{/snippet}
	</AreaChart>
</Chart.Container>

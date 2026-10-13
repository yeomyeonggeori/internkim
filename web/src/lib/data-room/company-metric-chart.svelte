<script lang="ts">
	import * as Chart from '$lib/components/ui/chart';
	import { AreaChart } from 'layerchart';
	import { formatMetricValue, periodLabel, type CompanyMetricSeries } from './company';

	let { series, label, locale }: { series: CompanyMetricSeries; label: string; locale: string } = $props();

	const chartData = $derived(series.points.map((point, index) => ({ index, value: point.value })));
	const chartConfig = $derived<Chart.ChartConfig>({ value: { label, color: 'var(--color-foreground)' } });
	const edgeTicks = $derived([0, series.points.length - 1]);

	function pointLabel(value: unknown): string {
		const point = series.points[Number(value)];
		return point ? periodLabel(point, locale) : '';
	}
</script>

<Chart.Container config={chartConfig} class="h-20 w-full" aria-label={label}>
	<AreaChart
		data={chartData}
		x="index"
		y="value"
		axis="x"
		rule={false}
		series={[{ key: 'value', label, color: 'var(--color-foreground)' }]}
		padding={{ left: 4, right: 4, bottom: 16 }}
		props={{
			xAxis: { ticks: edgeTicks, format: pointLabel },
			area: { fillOpacity: 0.12, line: { strokeWidth: 2 } }
		}}
	>
		{#snippet tooltip()}
			<Chart.Tooltip labelFormatter={pointLabel} indicator="dot" motion="none">
				{#snippet formatter({ value })}
					<span class="text-foreground font-medium tabular-nums">
						{typeof value === 'number' ? formatMetricValue(series.latest, locale, value) : String(value)}
					</span>
				{/snippet}
			</Chart.Tooltip>
		{/snippet}
	</AreaChart>
</Chart.Container>

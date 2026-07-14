<script lang="ts">
	import * as Chart from '$lib/components/ui/chart';
	import { AreaChart } from 'layerchart';
	import type { CompanyShareActivityDay } from './company-page-model';

	type ActivityChartText = {
		attendanceSignals: string;
		workSignals: string;
	};

	let { days, language, text }: { days: CompanyShareActivityDay[]; language: 'ko' | 'en'; text: ActivityChartText } = $props();
	const chartData = $derived(days.map((day, index) => ({ ...day, index })));
	const chartConfig = $derived<Chart.ChartConfig>({
		attendanceCount: { label: text.attendanceSignals, color: 'var(--color-blue-400)' },
		workCount: { label: text.workSignals, color: 'var(--color-blue-600)' }
	});
	const chartSeries = $derived([
		{ key: 'attendanceCount', label: text.attendanceSignals, color: 'var(--color-blue-400)' },
		{ key: 'workCount', label: text.workSignals, color: 'var(--color-blue-600)' }
	]);
	const tooltipLabels = $derived(new Map(chartData.map((day) => [day.index, formatDay(day.date)])));
	const axisLabelIndexes = $derived(createAxisLabelIndexes(chartData.length));

	function formatDay(date: string): string {
		return new Intl.DateTimeFormat(language, { month: 'short', day: 'numeric' }).format(new Date(`${date}T00:00:00`));
	}

	function formatTooltipLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		return tooltipLabels.get(index) ?? String(value);
	}

	function formatAxisLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		if (!axisLabelIndexes.includes(index)) return '';
		return chartData[index] ? formatDay(chartData[index].date) : '';
	}

	function createAxisLabelIndexes(count: number): number[] {
		if (count <= 1) return [0];
		return Array.from({ length: 5 }, (_, index) => Math.round(index * (count - 1) / 4));
	}
</script>

<Chart.Container config={chartConfig} class="h-44 w-full" aria-label={language === 'ko' ? '최근 30일 팀 활동' : 'Team activity over the last 30 days'}>
	<AreaChart
		data={chartData}
		x="index"
		axis="x"
		grid
		rule={false}
		series={chartSeries}
		seriesLayout="stack"
		padding={{ left: 6, right: 6, bottom: 18 }}
		props={{
			grid: { class: 'stroke-border/60' },
			xAxis: { ticks: axisLabelIndexes, format: formatAxisLabel },
			area: { fillOpacity: 0.16, line: { strokeWidth: 2.5 } }
		}}
	>
		{#snippet tooltip()}
			<Chart.Tooltip
				anchor="bottom"
				contained={false}
				indicator="dot"
				labelFormatter={formatTooltipLabel}
				class="w-max min-w-40"
				motion="none"
				x="data"
				y="data"
				yOffset={8}
			>
				{#snippet formatter({ value, name, item })}
					<div class="size-2.5 shrink-0 rounded-[2px]" style={`background-color:${item.color}`}></div>
					<div class="flex flex-1 items-center justify-between gap-5 leading-none">
						<span class="text-muted-foreground whitespace-nowrap">{name}</span>
						<span class="text-foreground font-mono font-medium tabular-nums">{typeof value === 'number' ? value : String(value)}</span>
					</div>
				{/snippet}
			</Chart.Tooltip>
		{/snippet}
	</AreaChart>
</Chart.Container>

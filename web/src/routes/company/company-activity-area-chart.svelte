<script lang="ts">
	import * as Chart from '$lib/components/ui/chart';
	import { AreaChart } from 'layerchart';
	import {
		companyActivityNormalizedValue,
		companyActivityScaleMaximum,
		formatCompanyActivityDuration,
		type CompanyShareActivityDay
	} from './company-page-model';

	type ActivityChartText = {
		workHours: string;
		workSignals: string;
	};

	let { days, language, text }: { days: CompanyShareActivityDay[]; language: string; text: ActivityChartText } = $props();
	const maximumWorkCount = $derived(companyActivityScaleMaximum(days.map((day) => day.workCount)));
	const maximumWorkMinutes = $derived(companyActivityScaleMaximum(days.map((day) => day.workMinutes ?? 0)));
	const chartData = $derived(days.map((day, index) => ({
		...day,
		index,
		workCountNormalized: companyActivityNormalizedValue(day.workCount, maximumWorkCount),
		workMinutesNormalized: companyActivityNormalizedValue(day.workMinutes ?? 0, maximumWorkMinutes)
	})));
	const chartConfig = $derived<Chart.ChartConfig>({
		workCountNormalized: { label: text.workSignals, color: 'var(--color-blue-600)' },
		workMinutesNormalized: { label: text.workHours, color: 'var(--color-blue-400)' }
	});
	const chartSeries = $derived([
		{ key: 'workCountNormalized', label: text.workSignals, color: 'var(--color-blue-600)' },
		{ key: 'workMinutesNormalized', label: text.workHours, color: 'var(--color-blue-400)' }
	]);
	const tooltipLabels = $derived(new Map(chartData.map((day) => [day.index, formatDay(day.date)])));
	const axisLabelIndexes = $derived(createAxisLabelIndexes(chartData.length));
	const axisRatios = [1, 0.5, 0];

	function formatDay(date: string): string {
		return new Intl.DateTimeFormat(language, { month: 'short', day: 'numeric' }).format(new Date(`${date}T00:00:00`));
	}

	function formatTooltipLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		return tooltipLabels.get(index) ?? String(value);
	}

	function formatTooltipValue(value: unknown, name: string): string {
		const normalizedValue = typeof value === 'number' ? value : Number(value);
		if (!Number.isFinite(normalizedValue)) return String(value);
		if (name === text.workHours) return formatCompanyActivityDuration(normalizedValue / 100 * maximumWorkMinutes, language);
		return new Intl.NumberFormat(language).format(Math.round(normalizedValue / 100 * maximumWorkCount));
	}

	function formatAxisLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		if (!axisLabelIndexes.includes(index)) return '';
		return chartData[index] ? formatDay(chartData[index].date) : '';
	}

	function formatWorkCountAxis(ratio: number): string {
		return new Intl.NumberFormat(language).format(Math.round(maximumWorkCount * ratio));
	}

	function formatWorkHoursAxis(ratio: number): string {
		return formatCompanyActivityDuration(maximumWorkMinutes * ratio, language);
	}

	function createAxisLabelIndexes(count: number): number[] {
		if (count <= 1) return [0];
		return Array.from({ length: 5 }, (_, index) => Math.round(index * (count - 1) / 4));
	}
</script>

<div class="relative">
	<div class="text-muted-foreground pointer-events-none absolute inset-y-2 left-0 z-10 flex w-9 flex-col justify-between pb-6 text-left text-[10px] tabular-nums" aria-hidden="true">
		{#each axisRatios as ratio}<span>{formatWorkCountAxis(ratio)}</span>{/each}
	</div>
	<div class="text-muted-foreground pointer-events-none absolute inset-y-2 right-0 z-10 flex w-10 flex-col items-end justify-between pb-6 text-right text-[10px] tabular-nums" aria-hidden="true">
		{#each axisRatios as ratio}<span>{formatWorkHoursAxis(ratio)}</span>{/each}
	</div>
	<Chart.Container config={chartConfig} class="h-48 w-full" aria-label={language === 'ko' ? '최근 30일 업무 갱신과 총 근무시간' : 'Work updates and total work hours over the last 30 days'}>
		<AreaChart
			data={chartData}
			x="index"
			yDomain={[0, 100]}
			axis="x"
			grid
			rule={false}
			series={chartSeries}
			seriesLayout="overlap"
			padding={{ left: 38, right: 42, bottom: 18 }}
			props={{
				grid: { class: 'stroke-border/60' },
				xAxis: { ticks: axisLabelIndexes, format: formatAxisLabel },
				area: { fillOpacity: 0.14, line: { strokeWidth: 2.5 } }
			}}
		>
			{#snippet tooltip()}
				<Chart.Tooltip
					anchor="bottom"
					contained={false}
					indicator="dot"
					labelFormatter={formatTooltipLabel}
					class="w-max min-w-44"
					motion="none"
					x="data"
					y="data"
					yOffset={8}
				>
					{#snippet formatter({ value, name, item })}
						<div class="size-2.5 shrink-0 rounded-[2px]" style={`background-color:${item.color}`}></div>
						<div class="flex flex-1 items-center justify-between gap-5 leading-none">
							<span class="text-muted-foreground whitespace-nowrap">{name}</span>
							<span class="text-foreground font-mono font-medium tabular-nums">{formatTooltipValue(value, name)}</span>
						</div>
					{/snippet}
				</Chart.Tooltip>
			{/snippet}
		</AreaChart>
	</Chart.Container>
</div>

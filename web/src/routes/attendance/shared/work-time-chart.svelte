<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import type * as Chart from '$lib/components/ui/chart';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { Snippet } from 'svelte';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import { getAttendanceState, type ChartMode } from '../attendance-context.svelte';
	import { loadedWorkTimeChartPlot, loadWorkTimeChartPlot } from './work-time-chart-plot-loader';
	import { attendanceText } from '../text';
	import { todayDateInTimeZone } from './attendance-date';
	import DurationText from './duration-text.svelte';
	import {
		buildSeries,
		summarizeDailyValues,
		type ChartPoint,
		type DailyValue,
		type WorkTimeChartLocation,
	} from './work-time-chart-model';

	type Props = {
		title: string;
		dailyValues: DailyValue[];
		locations: WorkTimeChartLocation[];
		formatValue: (value: number) => string;
		compact?: boolean;
		footer?: Snippet;
	};

	const fallbackLocationColor = 'var(--color-muted-foreground)';

	type ChartModeOption = {
		value: ChartMode;
		label: string;
	};

	let {
		title,
		dailyValues,
		locations,
		formatValue,
		compact = false,
		footer,
	}: Props = $props();

	const attendance = getAttendanceState();
	let plotComponent = $state(loadedWorkTimeChartPlot());

	void loadWorkTimeChartPlot().then((plot) => {
		plotComponent = plot;
	});
	const text = createPageText(attendanceText);
	const modes = $derived<ChartModeOption[]>([
		{ value: 'day', label: text.day },
		{ value: 'week', label: text.week },
		{ value: 'month', label: text.month },
	]);

	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const series = $derived(
		buildSeries(
			attendance.summary?.month ?? '',
			dailyValues,
			attendance.chartMode,
			{
				weekLabelTemplate: text.weekLabelTemplate,
				monthLabelTemplate: text.monthLabelTemplate,
				total: text.total,
				weekdaySunday: text.weekdaySunday,
				weekdayMonday: text.weekdayMonday,
				weekdayTuesday: text.weekdayTuesday,
				weekdayWednesday: text.weekdayWednesday,
				weekdayThursday: text.weekdayThursday,
				weekdayFriday: text.weekdayFriday,
				weekdaySaturday: text.weekdaySaturday,
			},
			{ today }
		)
	);

	const chartData = $derived(series.map((point, index) => ({ ...point, index })));
	const chartConfig = $derived<Chart.ChartConfig>(
		Object.fromEntries(
			locations.map((location) => [
				location.key,
				{ label: location.name, color: location.color ?? fallbackLocationColor },
			])
		)
	);
	const chartSeries = $derived(
		locations.map((location) => ({
			key: location.key,
			label: location.name,
			value: (point: ChartPoint) => {
				const minutes = point.values[location.key] ?? 0;
				return minutes > 0 ? minutes : undefined;
			},
			color: location.color ?? fallbackLocationColor,
		}))
	);
	const tooltipLabelByIndex = $derived(new Map(chartData.map((point) => [point.index, point.tooltipLabel])));
	const axisLabelIndexes = $derived(createAxisLabelIndexes(attendance.chartMode, chartData.length));
	const axisLabelIndexSet = $derived(new Set(axisLabelIndexes));
	const axisTickLabelProps = $derived(chartData.length === 1 ? { textAnchor: 'start' as const } : undefined);
	const isPercentMode = $derived(attendance.chartMode !== 'day');
	const maxTotal = $derived(
		isPercentMode ? 100 : Math.max(1, ...chartData.map((point) => point.totalMinutes))
	);
	const pointsSummary = $derived(summarizeDailyValues(dailyValues, { today }));

	function formatChartValue(value: number): string {
		if (isPercentMode) return `${Math.round(value)}%`;
		return formatValue(value);
	}

	function createAxisLabelIndexes(mode: ChartMode, count: number): number[] {
		if (mode === 'month') return [0, 3, 6, 9, 11].filter((index) => index < count);
		const maximumLabelCount = mode === 'week' ? 6 : 5;
		if (count <= maximumLabelCount) {
			return Array.from({ length: count }, (_, index) => index);
		}
		const lastIndex = count - 1;
		return Array.from({ length: maximumLabelCount }, (_, index) =>
			Math.round((index * lastIndex) / (maximumLabelCount - 1))
		);
	}

	function tooltipLabelFormatter(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		return tooltipLabelByIndex.get(index) ?? String(value);
	}

	function axisLabelFormatter(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		if (!axisLabelIndexSet.has(index)) return '';
		return chartData[index]?.label ?? '';
	}
</script>

<Card.Root class={compact ? 'gap-2' : undefined} aria-label={title}>
	<Card.Header class={compact ? 'flex flex-col gap-2 space-y-0 pb-0' : 'flex flex-row items-center justify-between space-y-0'}>
		<Card.Title class="flex items-center gap-1.5 text-sm">
			<ClockIcon class="size-3.5 text-muted-foreground" />
			{title}
		</Card.Title>
		<div class={compact ? 'grid w-full grid-cols-3 gap-1 rounded-md border p-0.5' : 'flex items-center gap-1 rounded-md border p-0.5'}>
			{#each modes as mode (mode.value)}
				<Button
					variant={attendance.chartMode === mode.value ? 'default' : 'ghost'}
					size="sm"
					class={compact ? 'h-6 w-full px-0 text-[10px]' : 'h-7 px-3 text-xs'}
					onclick={() => (attendance.chartMode = mode.value)}
				>
					{mode.label}
				</Button>
			{/each}
		</div>
	</Card.Header>
	<Card.Content class={compact ? 'px-3 pb-3 pt-1' : undefined}>
		{#if !plotComponent}
			<Skeleton class={compact ? 'h-24 w-full' : 'h-72 w-full'} />
		{:else}
			{@const WorkTimeChartPlot = plotComponent}
			<WorkTimeChartPlot
				{title}
				{compact}
				{chartData}
				{chartConfig}
				{chartSeries}
				{maxTotal}
				{axisLabelIndexes}
				{axisTickLabelProps}
				{fallbackLocationColor}
				{axisLabelFormatter}
				{tooltipLabelFormatter}
				{formatChartValue}
			/>
		{/if}
		{#if pointsSummary}
			<div class={compact ? 'mt-2 grid gap-1 border-t pt-2 text-[11px]' : 'mt-3 flex items-center gap-6 text-xs'}>
				<div class="flex items-baseline justify-between gap-2">
					<span class="whitespace-nowrap text-muted-foreground">{text.dailyAverage}</span>
					<DurationText minutes={pointsSummary.averageMinutes} size="inherit" tone="default" />
				</div>
				<div class="flex items-baseline justify-between gap-2">
					<span class="whitespace-nowrap text-muted-foreground">{text.dailyMaximum}</span>
					<DurationText minutes={pointsSummary.maximumMinutes} size="inherit" tone="default" />
				</div>
				<div class="flex items-baseline justify-between gap-2">
					<span class="whitespace-nowrap text-muted-foreground">{text.dailyMinimum}</span>
					<DurationText minutes={pointsSummary.minimumMinutes} size="inherit" tone="default" />
				</div>
			</div>
		{/if}
		{#if footer}
			{@render footer()}
		{/if}
	</Card.Content>
</Card.Root>

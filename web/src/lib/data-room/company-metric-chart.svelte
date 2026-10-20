<script lang="ts">
	import { cn } from '$lib/utils';
	import { formatMetricValue, metricBars, type CompanyMetricSeries, type MetricBar } from './company';

	let { series, label, locale }: { series: CompanyMetricSeries; label: string; locale: string } = $props();

	const shownPeriods = 5;
	const chart = $derived(metricBars(series, shownPeriods, locale));
	const smallestVisibleBar = '4px';

	function barHeight(bar: MetricBar): string {
		if (bar.value === 0) return '0';
		return `max(${smallestVisibleBar}, ${bar.share * 100}%)`;
	}
</script>

<figure class="grid gap-1.5" aria-label={label}>
	<div class="grid gap-2" style:grid-template-columns={`repeat(${chart.bars.length}, minmax(0, 1fr))`}>
		{#each chart.bars as bar (bar.period)}
			<div class="grid min-w-0 gap-1.5">
				<span class={cn('truncate text-center text-xs tabular-nums', bar.isLatest ? 'text-foreground font-medium' : 'text-muted-foreground')}>
					{formatMetricValue(series.latest, locale, bar.value)}
				</span>
				<div class="grid h-20" style:grid-template-rows={`${chart.positiveShare}fr ${chart.negativeShare}fr`}>
					<div class="flex items-end">
						{#if bar.value > 0}
							<span class={cn('w-full rounded-t-[3px]', bar.isLatest ? 'bg-foreground' : 'bg-foreground/15')} style:height={barHeight(bar)}></span>
						{/if}
					</div>
					{#if chart.negativeShare > 0}
						<div class="border-foreground/25 flex items-start border-t">
							{#if bar.value < 0}
								<span class={cn('w-full rounded-b-[3px]', bar.isLatest ? 'bg-foreground' : 'bg-foreground/15')} style:height={barHeight(bar)}></span>
							{/if}
						</div>
					{/if}
				</div>
				<span class="text-muted-foreground truncate text-center text-xs tabular-nums">{bar.period}</span>
			</div>
		{/each}
	</div>
</figure>

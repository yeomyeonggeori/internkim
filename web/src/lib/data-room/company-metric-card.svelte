<script lang="ts">
	import CompanyMetricChart from './company-metric-chart.svelte';
	import { dataRoomCompanyText } from './company-text';
	import { formatMetricValue, humanizedKey, isProjected, periodLabel, periodYear, subPeriodLabel, type CompanyMetricSeries } from './company';

	let { series, locale, note, projectionNote, span }: { series: CompanyMetricSeries; locale: 'ko' | 'en'; note: string; projectionNote: string; span: number } = $props();

	const text = $derived(dataRoomCompanyText[locale]);
	const label = $derived(text.metricNames[series.metric] ?? humanizedKey(series.metric));
	const isOnlyProjected = $derived(isProjected(series.latest));
	const chartedPeriods = $derived(series.points.length + (series.projection ? 1 : 0));
</script>

{#snippet projectedTag()}<span class="border-foreground/30 text-muted-foreground mr-1.5 rounded-sm border border-dashed px-1 py-px text-[11px]">{text.projected}</span>{/snippet}

<article class="bg-background grid content-between gap-4 p-5" style:grid-column={span > 1 ? `span ${span}` : undefined}>
	<div class="grid gap-1">
		<p class="text-muted-foreground text-xs">{label}</p>
		<p class="text-2xl font-semibold tracking-tight tabular-nums" class:text-muted-foreground={isOnlyProjected}>{formatMetricValue(series.latest, locale)}</p>
		<p class="text-muted-foreground text-xs tabular-nums">
			{#if isOnlyProjected}{@render projectedTag()}{/if}{periodLabel(series.latest, locale)}{#if note}<span class="text-foreground ml-2 font-medium">{note}</span>{/if}
		</p>
		{#if series.projection}
			<p class="text-muted-foreground mt-1 text-xs tabular-nums">
				{@render projectedTag()}{periodLabel(series.projection, locale)}
				<span class="text-foreground ml-1 font-medium">{formatMetricValue(series.projection, locale)}</span>
				{#if projectionNote}<span class="ml-1.5">{projectionNote}</span>{/if}
			</p>
		{/if}
		{#if series.interim.length > 0}
			<p class="text-muted-foreground text-xs tabular-nums">
				{text.soFar(periodYear(series.interim[0]))}
				{#each series.interim as point, index (subPeriodLabel(point, locale))}{index > 0 ? ' · ' : ' '}{subPeriodLabel(point, locale)} <span class="text-foreground font-medium">{formatMetricValue(point, locale)}</span>{/each}
			</p>
		{/if}
	</div>
	{#if chartedPeriods > 1}<CompanyMetricChart {series} {label} {locale} />{/if}
</article>

<script lang="ts">
	import CompanyMetricChart from './company-metric-chart.svelte';
	import { dataRoomCompanyText } from './company-text';
	import { formatMetricValue, humanizedKey, periodLabel, type CompanyMetricSeries } from './company';

	let { series, locale, note, span }: { series: CompanyMetricSeries; locale: 'ko' | 'en'; note: string; span: number } = $props();

	const label = $derived(dataRoomCompanyText[locale].metricNames[series.metric] ?? humanizedKey(series.metric));
</script>

<article class="bg-background grid content-between gap-4 p-5" style:grid-column={span > 1 ? `span ${span}` : undefined}>
	<div class="grid gap-1">
		<p class="text-muted-foreground text-xs">{label}</p>
		<p class="text-2xl font-semibold tracking-tight tabular-nums">{formatMetricValue(series.latest, locale)}</p>
		<p class="text-muted-foreground text-xs tabular-nums">
			{periodLabel(series.latest, locale)}{#if note}<span class="text-foreground ml-2 font-medium">{note}</span>{/if}
		</p>
	</div>
	{#if series.points.length > 1}<CompanyMetricChart {series} {label} {locale} />{/if}
</article>

<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import FlowBusinessDistanceDonut from './flow-business-distance-donut.svelte';
	import FlowDailyTypeDistanceChart from './flow-daily-type-distance-chart.svelte';
	import FlowDistanceLineChart from './flow-distance-line-chart.svelte';
	import type { FlowChartSection } from './flow-report-data';

	type Props = {
		section: FlowChartSection;
	};

	let { section }: Props = $props();

	function isCompactCard(): boolean {
		return section.chartKind === 'dailyTypeStacked';
	}

	function isSectionEmpty(): boolean {
		if (section.chartKind === 'lineComparison') return section.trend.currentValues.length === 0 && section.trend.previousValues.length === 0;
		return section.items.length === 0;
	}

	function cardRootClass(): string {
		const heightClass = section.chartKind === 'lineComparison' ? 'min-h-[32rem]' : section.chartKind === 'dailyTypeStacked' ? 'min-h-[23rem]' : 'min-h-[18rem]';
		return `${heightClass} flex min-w-0 w-full flex-col`;
	}

	function cardHeaderClass(): string {
		return isCompactCard() ? 'pb-1' : 'pb-3';
	}
</script>

<Card.Root size={isCompactCard() ? 'sm' : 'default'} class={cardRootClass()}>
	<Card.Header class={cardHeaderClass()}>
		<div class="min-w-0 space-y-1">
			<Card.Title class="text-base">{section.title}</Card.Title>
			{#if section.description}
				<Card.Description class="text-sm leading-snug">{section.description}</Card.Description>
			{/if}
		</div>
	</Card.Header>
	<Card.Content class="min-h-0 flex-1">
		{#if isSectionEmpty()}
			<div class="grid min-h-36 place-items-center rounded-md border border-dashed bg-muted/20 px-4 text-sm text-muted-foreground">
				{section.emptyLabel}
			</div>
		{:else if section.chartKind === 'dailyTypeStacked'}
			<FlowDailyTypeDistanceChart section={section} />
		{:else if section.chartKind === 'lineComparison'}
			<FlowDistanceLineChart section={section} variant={section.id === 'monthlyDistanceTrend' ? 'monthly' : 'weekly'} />
		{:else if section.chartKind === 'donut'}
			<FlowBusinessDistanceDonut section={section} />
		{/if}
	</Card.Content>
</Card.Root>

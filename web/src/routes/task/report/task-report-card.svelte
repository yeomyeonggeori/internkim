<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import TaskBusinessDistanceDonut from './task-business-distance-donut.svelte';
	import TaskDailyTypeDistanceChart from './task-daily-type-distance-chart.svelte';
	import TaskDistanceLineChart from './task-distance-line-chart.svelte';
	import type { TaskChartSection } from './task-report-data';
	import type { TaskDefinitions } from '../task-types';

	type Props = {
		section: TaskChartSection;
		definitions?: TaskDefinitions;
		showEmpty?: boolean;
	};

	let { section, definitions, showEmpty = true }: Props = $props();

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
			{#if showEmpty}<Empty.Root><Empty.Header><Empty.Title>{section.emptyLabel}</Empty.Title></Empty.Header></Empty.Root>{/if}
		{:else if section.chartKind === 'dailyTypeStacked'}
			<TaskDailyTypeDistanceChart section={section} {definitions} {showEmpty} />
		{:else if section.chartKind === 'lineComparison'}
			<TaskDistanceLineChart section={section} variant={section.id === 'monthlyDistanceTrend' ? 'monthly' : 'weekly'} />
		{:else if section.chartKind === 'donut'}
			<TaskBusinessDistanceDonut section={section} {definitions} />
		{/if}
	</Card.Content>
</Card.Root>

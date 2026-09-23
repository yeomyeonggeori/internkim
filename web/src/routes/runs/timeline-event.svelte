<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import CircleSmallIcon from '@lucide/svelte/icons/circle-small';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import WrenchIcon from '@lucide/svelte/icons/wrench';
	import type { Snippet } from 'svelte';
	import type { EventLane } from './runs-api';

	let {
		value,
		lane,
		laneLabel,
		title,
		meta = '',
		elapsed,
		isOpen,
		children
	}: {
		value: string;
		lane: EventLane;
		laneLabel: string;
		title: string;
		meta?: string;
		elapsed: string;
		isOpen: boolean;
		children: Snippet;
	} = $props();

	const laneIcons = { llm: SparklesIcon, tool: WrenchIcon, failure: TriangleAlertIcon, other: CircleSmallIcon };
	const LaneIcon = $derived(laneIcons[lane]);
</script>

<Accordion.Item {value}>
	<Accordion.Trigger class="items-center gap-3 px-3">
		<span title={laneLabel} class="shrink-0">
			<LaneIcon aria-hidden="true" class="size-4 {lane === 'failure' ? 'text-destructive' : 'text-muted-foreground'}" />
			<span class="sr-only">{laneLabel}</span>
		</span>
		<code class="min-w-0 flex-1 truncate text-xs font-normal">{title}</code>
		{#if meta}
			<span class="hidden shrink-0 text-xs font-normal text-muted-foreground tabular-nums sm:inline">{meta}</span>
		{/if}
		<span class="w-14 shrink-0 text-right text-xs font-normal text-muted-foreground tabular-nums">{elapsed}</span>
	</Accordion.Trigger>
	<Accordion.Content class="flex flex-col gap-3 px-3">
		{#if isOpen}
			{@render children()}
		{/if}
	</Accordion.Content>
</Accordion.Item>

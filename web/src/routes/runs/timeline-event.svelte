<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import type { Component, Snippet } from 'svelte';
	import CircleDotIcon from '@lucide/svelte/icons/circle-dot';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import WrenchIcon from '@lucide/svelte/icons/wrench';
	import XIcon from '@lucide/svelte/icons/x';
	import type { EventLane } from './runs-api';
	import { formatEventClock } from './runs-view';

	let {
		value,
		title,
		duration = '',
		cost = '',
		createdAt,
		isFailed = false,
		lane,
		isOpen,
		children
	}: {
		value: string;
		title: string;
		duration?: string;
		cost?: string;
		createdAt?: string;
		isFailed?: boolean;
		lane: EventLane;
		isOpen: boolean;
		children: Snippet;
	} = $props();

	const laneIcons: Record<EventLane, Component> = { llm: SparklesIcon, tool: WrenchIcon, failure: XIcon, other: CircleDotIcon };
	const LaneIcon = $derived(isFailed ? XIcon : laneIcons[lane]);
	const namespaceEnd = $derived(title.indexOf('.') + 1);
</script>

<Accordion.Item {value}>
	<Accordion.Trigger>
		<span class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-4 gap-y-0.5 text-xs font-normal sm:grid-cols-[6rem_minmax(0,1fr)_auto]">
			<span class="order-3 col-start-1 text-muted-foreground tabular-nums sm:order-1 sm:col-start-auto">{formatEventClock(createdAt)}</span>
			<span class="order-1 flex min-w-0 items-center gap-2 sm:order-2 {isFailed ? 'text-destructive' : ''}">
				<LaneIcon aria-hidden="true" class="size-3.5 shrink-0 {isFailed ? '' : 'text-muted-foreground'}" />
				<code class="truncate"><span class={isFailed ? '' : 'text-muted-foreground'}>{title.slice(0, namespaceEnd)}</span>{title.slice(namespaceEnd)}</code>
			</span>
			{#if duration || cost}
				<span class="order-2 mr-2 flex justify-end gap-4 text-muted-foreground tabular-nums sm:order-3">
					<span class="w-12 text-right">{duration}</span>
					<span class="w-14 text-right">{cost}</span>
				</span>
			{/if}
		</span>
	</Accordion.Trigger>
	<Accordion.Content class="flex flex-col gap-3">
		{#if isOpen}
			{@render children()}
		{/if}
	</Accordion.Content>
</Accordion.Item>

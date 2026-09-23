<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import type { Snippet } from 'svelte';
	import { formatEventClock } from './runs-view';

	let {
		value,
		title,
		duration = '',
		cost = '',
		createdAt,
		isFailed = false,
		isOpen,
		children
	}: {
		value: string;
		title: string;
		duration?: string;
		cost?: string;
		createdAt?: string;
		isFailed?: boolean;
		isOpen: boolean;
		children: Snippet;
	} = $props();
</script>

<Accordion.Item {value}>
	<Accordion.Trigger>
		<span class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-4 gap-y-0.5 text-xs font-normal sm:grid-cols-[6rem_minmax(0,1fr)_auto]">
			<span class="order-3 col-start-1 text-muted-foreground tabular-nums sm:order-1 sm:col-start-auto">{formatEventClock(createdAt)}</span>
			<code class="order-1 truncate sm:order-2 {isFailed ? 'text-destructive' : ''}">{title}</code>
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

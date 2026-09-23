<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import type { Snippet } from 'svelte';
	import { formatEventClock } from './runs-view';

	let {
		value,
		title,
		meta = '',
		createdAt,
		isFailed = false,
		isOpen,
		children
	}: {
		value: string;
		title: string;
		meta?: string;
		createdAt?: string;
		isFailed?: boolean;
		isOpen: boolean;
		children: Snippet;
	} = $props();

	const details = $derived([formatEventClock(createdAt), meta].filter(Boolean).join(' · '));
</script>

<Accordion.Item {value}>
	<Accordion.Trigger>
		<span class="flex min-w-0 flex-1 flex-col gap-0.5">
			<code class="truncate text-xs font-normal {isFailed ? 'text-destructive' : ''}">{title}</code>
			<span class="text-xs font-normal text-muted-foreground tabular-nums">{details}</span>
		</span>
	</Accordion.Trigger>
	<Accordion.Content class="flex flex-col gap-3">
		{#if isOpen}
			{@render children()}
		{/if}
	</Accordion.Content>
</Accordion.Item>

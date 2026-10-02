<script lang="ts">
	import { MediaQuery } from 'svelte/reactivity';
	import type { Snippet } from 'svelte';
	import * as Sheet from '$lib/components/ui/sheet';

	let {
		isOpen,
		title,
		onClose,
		children
	}: {
		isOpen: boolean;
		title: string;
		onClose: () => void;
		children: Snippet;
	} = $props();
	const isDesktop = new MediaQuery('min-width: 1024px');
</script>

{#if isOpen && isDesktop.current}
	<aside
		aria-label={title}
		class="bg-card sticky top-0 flex h-[calc(100vh-9rem)] w-[26rem] shrink-0 flex-col overflow-hidden rounded-xl border shadow-sm"
	>
		{@render children()}
	</aside>
{/if}
<Sheet.Root
	open={isOpen && !isDesktop.current}
	onOpenChange={(open) => {
		if (!open) onClose();
	}}
>
	<Sheet.Content side="right" class="w-full gap-0 p-0 sm:max-w-md">
		<Sheet.Title class="sr-only">{title}</Sheet.Title>
		{#if isOpen}{@render children()}{/if}
	</Sheet.Content>
</Sheet.Root>

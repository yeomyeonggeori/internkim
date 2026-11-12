<script lang="ts">
	import type { Snippet } from 'svelte';
	import { providePlacement } from './workspace.svelte';

	let {
		pageID,
		title,
		description = '',
		children
	}: { pageID: string; title: string; description?: string; children: Snippet } = $props();

	providePlacement(() => ({ pageID, sectionName: title }));
</script>

<section class="border-foreground/10 bg-background/40 grid gap-6 rounded-sm border border-dashed p-6">
	<header class="grid gap-1">
		<h2 class="text-lg font-semibold tracking-tight">{title}</h2>
		{#if description}<p class="text-muted-foreground max-w-prose text-sm">{description}</p>{/if}
	</header>
	<div class="flex flex-wrap items-start gap-x-8 gap-y-10">
		{@render children()}
	</div>
</section>

<script lang="ts">
	import type { Snippet } from 'svelte';

	let { title, identifier, children, footer }: { title: string; identifier?: string; children: Snippet; footer?: Snippet } = $props();
</script>

<section class="min-w-0 overflow-hidden rounded-lg border">
	<header class="flex min-w-0 items-baseline gap-2 border-b bg-muted/40 px-3 py-2">
		<h3 class="shrink-0 text-xs font-medium text-muted-foreground">{title}</h3>
		{#if identifier}
			<code class="min-w-0 truncate text-xs">{identifier}</code>
		{/if}
	</header>
	<div class="flex min-w-0 flex-col gap-3 p-3">
		{@render children()}
	</div>
	{#if footer}
		<footer class="border-t px-3 py-2">
			{@render footer()}
		</footer>
	{/if}
</section>

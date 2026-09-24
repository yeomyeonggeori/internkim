<script lang="ts">
	import type { Fact } from './fact-list.svelte';

	let { facts }: { facts: Fact[] } = $props();
</script>

<dl class="grid grid-cols-3 overflow-hidden rounded-lg border bg-card lg:grid-flow-col lg:grid-cols-none lg:auto-cols-fr">
	{#each facts as fact (fact.label)}
		<div class="-mr-px -mb-px flex min-w-0 flex-col gap-1 border-r border-b px-3 py-2.5 sm:px-4 sm:py-3">
			<dt class="text-xs text-muted-foreground">{fact.label}</dt>
			<dd class="truncate text-sm font-medium tabular-nums sm:text-base" title={fact.value}>{fact.value}</dd>
		</div>
	{/each}
</dl>

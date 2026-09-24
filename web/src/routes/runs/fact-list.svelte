<script module lang="ts">
	export type Fact = { label: string; value: string; isIdentifier?: boolean };
</script>

<script lang="ts">
	let { facts }: { facts: Fact[] } = $props();
</script>

<dl class="flex flex-wrap gap-x-8 gap-y-3">
	{#each facts as fact (fact.label)}
		<div class="flex min-w-0 max-w-full flex-col gap-0.5">
			<dt class="text-xs text-muted-foreground">{fact.label}</dt>
			<dd class="truncate text-sm tabular-nums" title={fact.value}>
				{#if fact.isIdentifier}<code class="text-xs">{fact.value}</code>{:else}{fact.value}{/if}
			</dd>
		</div>
	{/each}
</dl>

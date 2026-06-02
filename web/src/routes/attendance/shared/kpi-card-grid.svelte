<script lang="ts">
	import type { Component } from 'svelte';
	import * as Card from '$lib/components/ui/card';

	export type KpiItem = {
		icon?: Component;
		iconClass?: string;
		label: string;
		value: string;
		sublabel?: string;
	};

	let { items }: { items: KpiItem[] } = $props();
</script>

<div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
	{#each items as item, index (index)}
		<Card.Root>
			<Card.Content class="space-y-2 p-4">
				<div class="flex items-center gap-1.5 text-xs text-muted-foreground">
					{#if item.icon}
						{@const Icon = item.icon}
						<Icon class={`size-3.5 ${item.iconClass ?? ''}`} />
					{/if}
					<span>{item.label}</span>
				</div>
				<p class="text-2xl font-semibold leading-tight">{item.value}</p>
				{#if item.sublabel}
					<p class="text-xs text-muted-foreground">{item.sublabel}</p>
				{/if}
			</Card.Content>
		</Card.Root>
	{/each}
</div>

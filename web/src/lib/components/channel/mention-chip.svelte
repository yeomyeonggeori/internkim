<script lang="ts">
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils.js';

	let { text, onclick, class: className = '', ...rest }: { text: string } & HTMLButtonAttributes = $props();

	const chipClass =
		'inline-block rounded bg-current/15 px-1 py-px leading-none font-semibold group-data-[variant=muted]/bubble:bg-background';
</script>

{#if onclick}
	<button
		{...rest}
		type="button"
		class={cn(chipClass, 'cursor-pointer hover:bg-current/25 group-data-[variant=muted]/bubble:hover:bg-[color-mix(in_oklch,var(--color-background),currentColor_12%)]', className)}
		{onclick}>{text}</button
	>
{:else}
	<span class={cn(chipClass, className)}>{text}</span>
{/if}

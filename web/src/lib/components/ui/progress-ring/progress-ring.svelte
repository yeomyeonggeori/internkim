<script lang="ts">
	import { cn, type WithElementRef } from '$lib/utils.js';
	import type { HTMLAttributes } from 'svelte/elements';

	const radius = 13;
	const circumference = 2 * Math.PI * radius;
	const indeterminateFraction = 0.25;

	let {
		ref = $bindable(null),
		class: className,
		value,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLSpanElement>> & { value: number | null } = $props();

	const percent = $derived(value === null ? null : Math.round(Math.min(100, Math.max(0, value))));
	const fillOffset = $derived(circumference * (1 - (percent === null ? indeterminateFraction : percent / 100)));
</script>

<span
	bind:this={ref}
	data-slot="progress-ring"
	role="progressbar"
	aria-valuemin={0}
	aria-valuemax={100}
	aria-valuenow={percent ?? undefined}
	class={cn('relative inline-flex size-8 shrink-0 items-center justify-center', className)}
	{...restProps}
>
	<svg
		viewBox="0 0 32 32"
		aria-hidden="true"
		class={cn('absolute inset-0 size-full -rotate-90', percent === null && 'animate-spin motion-reduce:animate-none')}
	>
		<circle cx="16" cy="16" r={radius} fill="none" stroke="currentColor" stroke-width="2.5" class="opacity-25" />
		<circle
			cx="16"
			cy="16"
			r={radius}
			fill="none"
			stroke="currentColor"
			stroke-width="2.5"
			stroke-linecap="round"
			stroke-dasharray={circumference}
			stroke-dashoffset={fillOffset}
			class="transition-[stroke-dashoffset] duration-150 ease-linear"
		/>
	</svg>
	{#if percent !== null}
		<span aria-hidden="true" class="font-mono text-[0.5625rem] leading-none tabular-nums">{percent}</span>
	{/if}
</span>

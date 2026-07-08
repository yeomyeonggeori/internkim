<script lang="ts">
	import { cn } from '$lib/utils';
	import { minidenticon } from 'minidenticons';

	let {
		seed,
		saturation = 90,
		lightness = 55,
		class: className
	}: {
		seed: string;
		saturation?: number;
		lightness?: number;
		class?: string;
	} = $props();

	const normalizedSeed = $derived((seed ?? '').trim().toLowerCase() || '?');
	const svg = $derived(minidenticon(normalizedSeed, saturation, lightness));
</script>

<div
	class={cn(
		'relative inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-background [&>svg]:absolute [&>svg]:inset-0 [&>svg]:size-full',
		className
	)}
	aria-hidden="true"
>
	{@html svg}
</div>

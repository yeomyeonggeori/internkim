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
	const svg = $derived(minidenticon(normalizedSeed, saturation, lightness).replace('<svg', '<svg class="size-full"'));
	const hue = $derived(Number(svg.match(/hsl\((\d+(?:\.\d+)?)/)?.[1] ?? 0));
</script>

<div
	class={cn(
		'relative inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full [&>svg]:absolute [&>svg]:inset-0 [&>svg]:size-full',
		'bg-[hsl(var(--identicon-hue)_var(--identicon-saturation)_92%)] dark:bg-[hsl(var(--identicon-hue)_var(--identicon-saturation)_14%)]',
		className
	)}
	style={`--identicon-hue: ${hue}; --identicon-saturation: ${saturation}%`}
	aria-hidden="true"
>
	{@html svg}
</div>

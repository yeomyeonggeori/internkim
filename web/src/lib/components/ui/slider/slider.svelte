<script lang="ts" module>
	export type SliderSize = "default" | "lg";
</script>

<script lang="ts">
	import { Slider as SliderPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		value = $bindable(),
		orientation = "horizontal",
		size = "default",
		class: className,
		...restProps
	}: WithoutChildrenOrChild<SliderPrimitive.RootProps> & { size?: SliderSize } = $props();

	const trackSize = {
		default: "data-horizontal:h-1 data-vertical:w-1",
		lg: "data-horizontal:h-6 data-vertical:w-6"
	};

	const thumbSize = {
		default: "size-3 border border-ring ring-ring/50 hover:ring-3 focus-visible:ring-3 active:ring-3",
		lg: "size-6 box-border touch-none border-[3px] border-[color:var(--slider-thumb-border,var(--color-border))] shadow-md"
	};
</script>

<!--
Discriminated Unions + Destructing (required for bindable) do not
get along, so we shut typescript up by casting `value` to `never`.
-->
<SliderPrimitive.Root
	bind:ref
	bind:value={value as never}
	data-slot="slider"
	{orientation}
	class={cn(
		"data-vertical:min-h-40 relative flex w-full touch-none items-center select-none data-disabled:opacity-50 data-vertical:h-full data-vertical:w-auto data-vertical:flex-col",
		className
	)}
	{...restProps}
>
	{#snippet children({ thumbItems })}
		<span
			data-slot="slider-track"
			data-orientation={orientation}
			class={cn(
				"rounded-full bg-muted relative grow overflow-hidden data-horizontal:w-full data-vertical:h-full",
				trackSize[size]
			)}
		>
			<SliderPrimitive.Range
				data-slot="slider-range"
				class={cn(
					"bg-primary absolute select-none data-horizontal:h-full data-vertical:w-full"
				)}
			/>
		</span>
		{#each thumbItems as thumb (thumb.index)}
			<SliderPrimitive.Thumb
				data-slot="slider-thumb"
				index={thumb.index}
				class={cn(
					"relative block shrink-0 select-none rounded-full bg-white transition-[color,box-shadow] after:absolute after:-inset-2 focus-visible:outline-hidden disabled:pointer-events-none disabled:opacity-50",
					thumbSize[size]
				)}
			/>
		{/each}
	{/snippet}
</SliderPrimitive.Root>

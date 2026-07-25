<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> = $props();
</script>

<div
	bind:this={ref}
	data-slot="message-avatar"
	class={cn(
		"min-w-8 group-has-data-[slot=message-footer]/message:-translate-y-8 bg-muted flex w-fit shrink-0 items-center justify-center self-end overflow-hidden rounded-full",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</div>

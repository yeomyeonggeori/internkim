<script lang="ts">
	import { cn, type WithElementRef, type WithoutChildren } from "$lib/utils.js";
	import MoreHorizontalIcon from "@lucide/svelte/icons/more-horizontal";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		label = "More pages",
		...restProps
	}: WithoutChildren<WithElementRef<HTMLAttributes<HTMLSpanElement>>> & { label?: string } = $props();
</script>

<span
	bind:this={ref}
	data-slot="pagination-ellipsis"
	aria-hidden="true"
	class={cn("flex size-9 items-center justify-center", className)}
	{...restProps}
>
	<MoreHorizontalIcon class="size-4" />
	<span class="sr-only">{label}</span>
</span>

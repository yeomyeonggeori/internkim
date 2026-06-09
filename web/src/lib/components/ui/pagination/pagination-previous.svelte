<script lang="ts">
	import { buttonVariants } from "$lib/components/ui/button/index.js";
	import { cn, type WithElementRef } from "$lib/utils.js";
	import ChevronLeftIcon from "@lucide/svelte/icons/chevron-left";
	import { Pagination as PaginationPrimitive, type WithoutChild } from "bits-ui";
	import type { ComponentProps } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";

	type PaginationPreviousProps = WithoutChild<ComponentProps<typeof PaginationPrimitive.PrevButton>>;

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<PaginationPreviousProps> = $props();
</script>

<PaginationPrimitive.PrevButton
	bind:ref
	data-slot="pagination-previous"
	{...restProps}
>
	{#snippet child({ props })}
		<button
			{...props}
			class={cn(
				buttonVariants({ variant: "ghost", size: "default" }),
				"gap-1 px-2.5",
				(props as HTMLButtonAttributes).class,
				className
			)}
		>
			{#if children}
				{@render children?.()}
			{:else}
				<ChevronLeftIcon class="size-4" />
				<span>Previous</span>
			{/if}
		</button>
	{/snippet}
</PaginationPrimitive.PrevButton>

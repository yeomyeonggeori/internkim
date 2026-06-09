<script lang="ts">
	import { buttonVariants } from "$lib/components/ui/button/index.js";
	import { cn, type WithElementRef } from "$lib/utils.js";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import { Pagination as PaginationPrimitive, type WithoutChild } from "bits-ui";
	import type { ComponentProps } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";

	type PaginationNextProps = WithoutChild<ComponentProps<typeof PaginationPrimitive.NextButton>>;

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<PaginationNextProps> = $props();
</script>

<PaginationPrimitive.NextButton
	bind:ref
	data-slot="pagination-next"
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
				<span>Next</span>
				<ChevronRightIcon class="size-4" />
			{/if}
		</button>
	{/snippet}
</PaginationPrimitive.NextButton>

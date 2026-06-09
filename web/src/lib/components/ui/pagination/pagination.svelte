<script lang="ts">
	import type { WithElementRef } from "$lib/utils.js";
	import { cn } from "$lib/utils.js";
	import { Pagination as PaginationPrimitive } from "bits-ui";
	import type { ComponentProps } from "svelte";
	import type { HTMLAttributes } from "svelte/elements";

	type PaginationProps = WithElementRef<ComponentProps<typeof PaginationPrimitive.Root>>;

	let {
		ref = $bindable(null),
		class: className,
		children,
		count,
		perPage,
		page = $bindable(),
		...restProps
	}: PaginationProps = $props();
</script>

<PaginationPrimitive.Root
	bind:ref
	bind:page
	{count}
	{perPage}
	data-slot="pagination"
	{...restProps}
>
	{#snippet child({ props, pages, range, currentPage })}
		<div
			bind:this={ref}
			{...props}
			class={cn("mx-auto flex w-full justify-center", (props as HTMLAttributes<HTMLDivElement>).class, className)}
		>
			{@render children?.({ pages, range, currentPage })}
		</div>
	{/snippet}
</PaginationPrimitive.Root>

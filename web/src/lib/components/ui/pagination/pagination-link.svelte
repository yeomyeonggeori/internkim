<script lang="ts">
	import { buttonVariants } from "$lib/components/ui/button/index.js";
	import { cn, type WithElementRef } from "$lib/utils.js";
	import { Pagination as PaginationPrimitive, type WithoutChild } from "bits-ui";
	import type { ComponentProps } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";

	type PaginationLinkProps = WithoutChild<ComponentProps<typeof PaginationPrimitive.Page>> &
		WithElementRef<{
			isActive?: boolean;
		}>;

	let {
		ref = $bindable(null),
		class: className,
		page,
		isActive,
		children,
		...restProps
	}: PaginationLinkProps = $props();
</script>

<PaginationPrimitive.Page
	bind:ref
	{page}
	data-slot="pagination-link"
	aria-current={isActive ? "page" : undefined}
	{...restProps}
>
	{#snippet child({ props })}
		<button
			{...props}
			class={cn(
				buttonVariants({ variant: isActive ? "outline" : "ghost", size: "icon" }),
				(props as HTMLButtonAttributes).class,
				className
			)}
		>
			{@render children?.()}
		</button>
	{/snippet}
</PaginationPrimitive.Page>

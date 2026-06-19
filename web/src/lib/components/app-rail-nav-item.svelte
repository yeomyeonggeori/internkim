<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import type { AppRailItem } from '$lib/components/app-rail-types';

	let {
		item,
		isActive = false,
		isExternal = false
	}: {
		item: AppRailItem;
		isActive?: boolean;
		isExternal?: boolean;
	} = $props();

	const railLinkBaseClass =
		'relative mx-2.5 flex h-10 w-10 items-center gap-3 overflow-hidden rounded-md bg-transparent px-2.5 text-sm font-medium text-sidebar-foreground/70 shadow-none transition-[width,color,background-color] duration-150 ease-out hover:bg-sidebar-accent hover:text-sidebar-accent-foreground hover:shadow-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring group-hover:w-[204px] group-focus-within:w-[204px] group-data-[profile-open=true]:w-[204px]';
	const railLinkActiveClass =
		'before:absolute before:left-0 before:top-1/2 before:h-6 before:w-0.5 before:-translate-y-1/2 before:rounded-r-full before:bg-sidebar-primary before:opacity-0 before:transition-opacity before:content-[\'\'] data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground data-[active=true]:before:opacity-100';
	const railLabelClass =
		'min-w-0 max-w-0 truncate opacity-0 transition-[max-width,opacity] duration-150 ease-out group-hover:max-w-[148px] group-hover:opacity-100 group-focus-within:max-w-[148px] group-focus-within:opacity-100 group-data-[profile-open=true]:max-w-[148px] group-data-[profile-open=true]:opacity-100';
	const linkClass = $derived(isExternal ? railLinkBaseClass : `${railLinkBaseClass} ${railLinkActiveClass}`);
	const Icon = $derived(item.icon);
</script>

<Sidebar.MenuItem>
	<Sidebar.MenuButton isActive={!isExternal && isActive} tooltipContent={item.label} class={linkClass}>
		{#snippet child({ props })}
			{#if isExternal}
				<a href={item.href} aria-label={item.label} target="_blank" rel="noopener noreferrer" {...props}>
					<Icon class="size-5! shrink-0" />
					<span class={railLabelClass}>{item.label}</span>
				</a>
			{:else}
				<a
					href={item.href}
					aria-label={item.label}
					data-sveltekit-preload-data="off"
					data-sveltekit-preload-code="off"
					{...props}
				>
					<Icon class="size-5! shrink-0" />
					<span class={railLabelClass}>{item.label}</span>
				</a>
			{/if}
		{/snippet}
	</Sidebar.MenuButton>
</Sidebar.MenuItem>

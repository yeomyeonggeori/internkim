<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import type { Snippet } from 'svelte';

	let {
		isProfileMenuOpen,
		children
	}: {
		isProfileMenuOpen: boolean;
		children?: Snippet;
	} = $props();

	let isRailFocusWithin = $state(false);
	let isRailHovered = $state(false);
	const isRailOpen = $derived(isRailHovered || isRailFocusWithin || isProfileMenuOpen);

	function handleRailFocusOut(event: FocusEvent) {
		const currentTarget = event.currentTarget;
		const relatedTarget = event.relatedTarget;
		if (currentTarget instanceof HTMLElement && relatedTarget instanceof Node && currentTarget.contains(relatedTarget)) return;
		isRailFocusWithin = false;
	}
</script>

<aside
	data-app-chrome
	class="group relative z-40 hidden h-svh w-[60px] shrink-0 md:block"
	data-profile-open={isProfileMenuOpen}
	onmouseenter={() => (isRailHovered = true)}
	onmouseleave={() => (isRailHovered = false)}
	onfocusin={() => (isRailFocusWithin = true)}
	onfocusout={handleRailFocusOut}
>
	<div data-app-rail class="absolute inset-y-0 left-0 flex w-[60px] flex-col gap-1 overflow-hidden border-r border-sidebar-border bg-sidebar py-2.5 transition-[width,box-shadow] duration-150 ease-out group-hover:w-[224px] group-hover:shadow-xl group-focus-within:w-[224px] group-focus-within:shadow-xl group-data-[profile-open=true]:w-[224px] group-data-[profile-open=true]:shadow-xl">
		<Sidebar.Provider open={isRailOpen} keyboardShortcutEnabled={false} class="contents">
			{@render children?.()}
		</Sidebar.Provider>
	</div>
</aside>

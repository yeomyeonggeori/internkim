<script lang="ts">
	import { appRailMode } from '$lib/components/app-rail-mode.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import type { Snippet } from 'svelte';

	let { children, onPointerInsideChange }: { children?: Snippet; onPointerInsideChange: (isInside: boolean) => void } = $props();

	function reportPointer(event: PointerEvent, isInside: boolean) {
		if (event.pointerType !== 'mouse') return;
		onPointerInsideChange(isInside);
	}
</script>

<Sidebar.Root
	collapsible="icon"
	data-app-chrome
	data-app-rail
	data-app-rail-mode={appRailMode.mode}
	class={appRailMode.mode === 'hover' ? 'group-data-[state=expanded]:shadow-lg' : ''}
	onpointerenter={(event: PointerEvent) => reportPointer(event, true)}
	onpointerleave={(event: PointerEvent) => reportPointer(event, false)}
>
	{@render children?.()}
	<Sidebar.Rail />
</Sidebar.Root>

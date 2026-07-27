<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import type { Snippet } from 'svelte';

	let { children }: { children?: Snippet } = $props();
</script>

<Sidebar.Root collapsible="icon" data-app-chrome>
	{@render children?.()}
	<Sidebar.Rail />
</Sidebar.Root>

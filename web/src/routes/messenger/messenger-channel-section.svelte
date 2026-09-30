<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		title,
		open,
		onOpenChange,
		action,
		children
	}: {
		title: string;
		open: boolean;
		onOpenChange: (open: boolean) => void;
		action?: Snippet;
		children: Snippet;
	} = $props();
</script>

<Collapsible.Root {open} {onOpenChange} class="group/collapsible">
	<Sidebar.Group>
		<Sidebar.GroupLabel>
			{#snippet child({ props })}
				<Collapsible.Trigger {...props}>
					{title}
					<ChevronRightIcon class="transition-transform group-data-[state=open]/collapsible:rotate-90" />
				</Collapsible.Trigger>
			{/snippet}
		</Sidebar.GroupLabel>
		{@render action?.()}
		<Collapsible.Content>
			<Sidebar.GroupContent>
				{@render children()}
			</Sidebar.GroupContent>
		</Collapsible.Content>
	</Sidebar.Group>
</Collapsible.Root>

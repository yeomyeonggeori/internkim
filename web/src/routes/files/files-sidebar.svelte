<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import UserIcon from '@lucide/svelte/icons/user';
	import UsersIcon from '@lucide/svelte/icons/users';
	import Share2Icon from '@lucide/svelte/icons/share-2';
	import { getFilesState } from './files-context.svelte';
	import type { WorkspaceRootKind } from './files-api';

	const files = getFilesState();

	const rootIcon: Record<WorkspaceRootKind, typeof UserIcon> = {
		personal: UserIcon,
		circle: UsersIcon,
		public: Share2Icon
	};
</script>

<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
	<nav class="min-h-0 flex-1 space-y-1 overflow-auto p-3">
		{#if files.isLoading && files.roots.length === 0}
			<div aria-hidden="true" class="grid gap-2">{#each [0, 1, 2] as row (row)}<Skeleton class="h-9 w-full" />{/each}</div>
		{/if}
		{#each files.roots as root (root.id)}
			{@const RootIcon = rootIcon[root.kind]}
			<Button
				variant={files.currentRoot?.id === root.id ? 'secondary' : 'ghost'}
				class="w-full justify-start gap-2"
				onclick={() => files.openRoot(root)}
			>
				<RootIcon class="size-4 shrink-0" />
				<span class="truncate">{root.label}</span>
			</Button>
		{/each}
	</nav>
</aside>

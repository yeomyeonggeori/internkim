<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UserIcon from '@lucide/svelte/icons/user';
	import UsersIcon from '@lucide/svelte/icons/users';
	import Share2Icon from '@lucide/svelte/icons/share-2';
	import { getFilesState } from './files-context.svelte';
	import type { WorkspaceRootKind } from './files-api';
	import { filesText } from './text';

	const text = createPageText(filesText);
	const files = getFilesState();

	const rootIcon: Record<WorkspaceRootKind, typeof UserIcon> = {
		personal: UserIcon,
		circle: UsersIcon,
		public: Share2Icon
	};
</script>

<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
	<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<FolderOpenIcon class="size-4 text-muted-foreground" />
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm font-medium">{text.title}</p>
			<p class="truncate text-xs text-muted-foreground">{text.subtitle}</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={() => files.refresh()}>
			<RefreshCwIcon class={files.isLoading ? 'animate-spin' : ''} />
		</Button>
	</div>

	<nav class="min-h-0 flex-1 space-y-1 overflow-auto p-3">
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

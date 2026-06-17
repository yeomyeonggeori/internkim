<script lang="ts">
	import * as TreeView from '$lib/components/ui/tree-view';
	import { Spinner } from '$lib/components/ui/spinner';
	import FileTreeNode from './file-tree-node.svelte';
	import { getFilesState } from './files-context.svelte';
	import { type WorkspaceEntry } from './files-api';

	let { entry }: { entry: WorkspaceEntry } = $props();

	const files = getFilesState();

	let open = $state(false);

	const children = $derived(files.childrenCache[entry.agentPath]);
	const isLoading = $derived(files.isLoadingPath(entry.agentPath));
	const isActiveDirectory = $derived(files.currentPath === entry.agentPath);
	const isSelectedFile = $derived(files.selectedFile?.agentPath === entry.agentPath);

	$effect(() => {
		if (open) files.loadChildren(entry.agentPath);
	});
</script>

{#if entry.isDirectory}
	<TreeView.Folder
		name={entry.name}
		bind:open
		class={isActiveDirectory ? 'text-primary font-medium' : ''}
		onclick={() => files.setActiveDirectory(entry.agentPath)}
	>
		{#if isLoading && !children}
			<Spinner class="text-muted-foreground my-1 ml-1 size-4" />
		{:else if children}
			{#each children as child (child.agentPath)}
				<FileTreeNode entry={child} />
			{/each}
		{/if}
	</TreeView.Folder>
{:else}
	<TreeView.File
		name={entry.name}
		class={isSelectedFile ? 'bg-accent rounded-sm font-medium' : ''}
		onclick={() => files.selectFile(entry)}
	/>
{/if}

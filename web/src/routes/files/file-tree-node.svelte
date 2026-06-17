<script lang="ts">
	import * as TreeView from '$lib/components/ui/tree-view';
	import { Spinner } from '$lib/components/ui/spinner';
	import FileTreeNode from './file-tree-node.svelte';
	import { listWorkspaceDirectory, workspaceDownloadURL, type WorkspaceEntry } from './files-api';

	let { entry }: { entry: WorkspaceEntry } = $props();

	let open = $state(false);
	let children = $state<WorkspaceEntry[] | null>(null);
	let isLoading = $state(false);
	let errorMessage = $state('');

	async function loadChildren() {
		if (children !== null || isLoading) return;
		isLoading = true;
		errorMessage = '';
		try {
			children = await listWorkspaceDirectory(entry.agentPath);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message.trim() : '';
		} finally {
			isLoading = false;
		}
	}

	function downloadFile() {
		const anchor = document.createElement('a');
		anchor.href = workspaceDownloadURL(entry.agentPath);
		anchor.download = entry.name;
		anchor.click();
	}

	$effect(() => {
		if (open) loadChildren();
	});
</script>

{#if entry.isDirectory}
	<TreeView.Folder name={entry.name} bind:open>
		{#if isLoading}
			<Spinner class="text-muted-foreground my-1 ml-1 size-4" />
		{:else if errorMessage}
			<span class="text-destructive py-1 pl-1 text-xs">{errorMessage}</span>
		{:else if children}
			{#each children as child (child.agentPath)}
				<FileTreeNode entry={child} />
			{/each}
		{/if}
	</TreeView.Folder>
{:else}
	<TreeView.File name={entry.name} onclick={downloadFile} />
{/if}

<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as TreeView from '$lib/components/ui/tree-view';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import FileTreeNode from './file-tree-node.svelte';
	import FileDetail from './file-detail.svelte';
	import { getFilesState } from './files-context.svelte';
	import { filesText } from './text';

	const text = createPageText(filesText);
	const files = getFilesState();

	let isDragging = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);

	function onDragOver(event: DragEvent) {
		event.preventDefault();
		isDragging = true;
	}

	function onDragLeave(event: DragEvent) {
		event.preventDefault();
		isDragging = false;
	}

	function onDrop(event: DragEvent) {
		event.preventDefault();
		isDragging = false;
		const dropped = event.dataTransfer?.files;
		if (dropped && dropped.length > 0) files.upload(Array.from(dropped));
	}

	function onFilePick(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		if (input.files && input.files.length > 0) files.upload(Array.from(input.files));
		input.value = '';
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center justify-end gap-3">
		<Button variant="outline" size="sm" disabled={files.isUploading} onclick={() => fileInput?.click()}>
			{#if files.isUploading}
				<Spinner />
			{:else}
				<UploadIcon />
			{/if}
			{files.isUploading ? text.uploading : text.upload}
		</Button>
		<input bind:this={fileInput} type="file" multiple class="hidden" onchange={onFilePick} />
	</div>

	{#if files.errorMessage}
		<Card.Root class="border-destructive/40 bg-destructive/5">
			<Card.Content class="flex items-center justify-between gap-3 py-3">
				<span class="text-sm text-destructive">{files.errorMessage}</span>
				<Button variant="outline" size="sm" onclick={() => files.reload()}>{text.retry}</Button>
			</Card.Content>
		</Card.Root>
	{/if}

	<div
		role="region"
		aria-label={text.title}
		ondragover={onDragOver}
		ondragleave={onDragLeave}
		ondrop={onDrop}
	>
		<Card.Root class={isDragging ? 'border-primary ring-primary/30 ring-2' : ''}>
			<Card.Content class="p-3">
				{#if files.isLoading}
					<div class="flex items-center justify-center py-12">
						<Spinner class="text-muted-foreground" />
					</div>
				{:else if files.entries.length === 0}
					<p class="text-muted-foreground py-12 text-center text-sm">{text.empty}</p>
				{:else}
					<TreeView.Root>
						{#each files.entries as entry (entry.agentPath)}
							<FileTreeNode {entry} />
						{/each}
					</TreeView.Root>
				{/if}
			</Card.Content>
			<Card.Footer class="border-t py-2">
				<p class="text-muted-foreground w-full text-center text-xs">{text.dropHint}</p>
			</Card.Footer>
		</Card.Root>
	</div>
</div>

<Sheet.Root
	open={files.selectedFile !== null}
	onOpenChange={(open) => {
		if (!open) files.clearSelection();
	}}
>
	<Sheet.Content side="right" class="gap-4">
		{#if files.selectedFile}
			<FileDetail file={files.selectedFile} />
		{/if}
	</Sheet.Content>
</Sheet.Root>

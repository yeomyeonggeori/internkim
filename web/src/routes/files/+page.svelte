<script lang="ts">
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import * as Card from '$lib/components/ui/card';
	import * as FileDropZone from '$lib/components/ui/file-drop-zone';
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
</script>

<div class="flex flex-col gap-4">
	{#if files.currentRoot}
		<Breadcrumb.Root>
			<Breadcrumb.List>
				{#each files.breadcrumbs as crumb, index (crumb.path)}
					{#if index > 0}
						<Breadcrumb.Separator />
					{/if}
					<Breadcrumb.Item>
						{#if index === files.breadcrumbs.length - 1}
							<Breadcrumb.Page class="max-w-[12rem] truncate">{crumb.label}</Breadcrumb.Page>
						{:else}
							<Breadcrumb.Link
								class="max-w-[12rem] cursor-pointer truncate"
								onclick={() => files.setActiveDirectory(crumb.path)}
							>
								{crumb.label}
							</Breadcrumb.Link>
						{/if}
					</Breadcrumb.Item>
				{/each}
			</Breadcrumb.List>
		</Breadcrumb.Root>
	{/if}

	{#if files.errorMessage}
		<Card.Root class="border-destructive/40 bg-destructive/5">
			<Card.Content class="flex items-center justify-between gap-3 py-3">
				<span class="text-destructive text-sm">{files.errorMessage}</span>
				<Button variant="outline" size="sm" onclick={() => files.reload()}>{text.retry}</Button>
			</Card.Content>
		</Card.Root>
	{/if}

	<Card.Root>
		<Card.Content class="p-3">
			{#if files.isLoading}
				<div class="flex items-center justify-center py-12">
					<Spinner class="text-muted-foreground" />
				</div>
			{:else if files.rootEntries.length === 0}
				<p class="text-muted-foreground py-12 text-center text-sm">{text.empty}</p>
			{:else}
				<TreeView.Root>
					{#each files.rootEntries as entry (entry.agentPath)}
						<FileTreeNode {entry} />
					{/each}
				</TreeView.Root>
			{/if}
		</Card.Content>
	</Card.Root>

	<FileDropZone.Root onUpload={(uploaded) => files.upload(uploaded)} disabled={files.isUploading}>
		<FileDropZone.Trigger>
			<div
				class="hover:bg-accent/25 flex flex-col place-items-center justify-center gap-2 rounded-lg border border-dashed p-6 transition-all hover:cursor-pointer"
			>
				{#if files.isUploading}
					<Spinner class="text-muted-foreground" />
				{:else}
					<UploadIcon class="text-muted-foreground size-6" />
				{/if}
				<span class="text-muted-foreground text-sm">{text.dropHint}</span>
			</div>
		</FileDropZone.Trigger>
	</FileDropZone.Root>
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

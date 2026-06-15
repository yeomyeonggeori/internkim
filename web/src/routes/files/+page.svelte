<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import { getFilesState } from './files-context.svelte';
	import { workspaceDownloadURL } from './files-api';
	import { formatFileSize, formatModifiedAt } from './files-path';
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
	<div class="flex flex-wrap items-center justify-between gap-3">
		<Breadcrumb.Root>
			<Breadcrumb.List>
				{#each files.breadcrumbs as crumb, index (crumb.path)}
					{#if index > 0}
						<Breadcrumb.Separator />
					{/if}
					<Breadcrumb.Item>
						{#if index === files.breadcrumbs.length - 1}
							<Breadcrumb.Page>{crumb.label}</Breadcrumb.Page>
						{:else}
							<Breadcrumb.Link class="cursor-pointer" onclick={() => files.navigateTo(crumb.path)}>
								{crumb.label}
							</Breadcrumb.Link>
						{/if}
					</Breadcrumb.Item>
				{/each}
			</Breadcrumb.List>
		</Breadcrumb.Root>

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
			<Card.Content class="p-0">
				{#if files.isLoading}
					<div class="flex items-center justify-center py-12">
						<Spinner class="text-muted-foreground" />
					</div>
				{:else if files.entries.length === 0}
					<p class="py-12 text-center text-sm text-muted-foreground">{text.empty}</p>
				{:else}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{text.name}</Table.Head>
								<Table.Head class="w-28 text-right">{text.size}</Table.Head>
								<Table.Head class="w-44">{text.modified}</Table.Head>
								<Table.Head class="w-12"></Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each files.entries as entry (entry.agentPath)}
								<Table.Row>
									<Table.Cell class="font-medium">
										{#if entry.isDirectory}
											<button
												type="button"
												class="flex items-center gap-2 hover:underline"
												onclick={() => files.navigateTo(entry.agentPath)}
											>
												<FolderIcon class="size-4 shrink-0 text-muted-foreground" />
												<span class="truncate">{entry.name}</span>
											</button>
										{:else}
											<div class="flex items-center gap-2">
												<FileIcon class="size-4 shrink-0 text-muted-foreground" />
												<span class="truncate font-normal">{entry.name}</span>
											</div>
										{/if}
									</Table.Cell>
									<Table.Cell class="text-right text-muted-foreground">
										{entry.isDirectory ? '—' : formatFileSize(entry.size)}
									</Table.Cell>
									<Table.Cell class="text-xs text-muted-foreground">
										{formatModifiedAt(entry.modifiedAt)}
									</Table.Cell>
									<Table.Cell class="text-right">
										{#if !entry.isDirectory}
											<Button
												variant="ghost"
												size="icon-sm"
												href={workspaceDownloadURL(entry.agentPath)}
												aria-label={text.download}
												download
											>
												<DownloadIcon />
											</Button>
										{/if}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				{/if}
			</Card.Content>
			<Card.Footer class="border-t py-2">
				<p class="w-full text-center text-xs text-muted-foreground">{text.dropHint}</p>
			</Card.Footer>
		</Card.Root>
	</div>
</div>

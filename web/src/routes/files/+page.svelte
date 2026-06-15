<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import { getFilesState } from './files-context.svelte';
	import { workspaceDownloadURL } from './files-api';
	import { formatFileSize } from './files-path';
	import { filesText } from './text';

	const text = createPageText(filesText);
	const files = getFilesState();

	let isDragging = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);

	function onDragOver(event: DragEvent) {
		event.preventDefault();
		isDragging = true;
	}

	function onDragLeave() {
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
		<nav class="flex min-w-0 flex-wrap items-center gap-1 text-sm">
			{#each files.breadcrumbs as crumb, index (crumb.path)}
				{#if index > 0}
					<ChevronRightIcon class="size-3.5 shrink-0 text-muted-foreground" />
				{/if}
				<button
					type="button"
					class="truncate rounded px-1.5 py-0.5 hover:bg-accent {index === files.breadcrumbs.length - 1
						? 'font-medium'
						: 'text-muted-foreground'}"
					onclick={() => files.navigateTo(crumb.path)}
				>
					{crumb.label}
				</button>
			{/each}
		</nav>

		<Button variant="outline" size="sm" disabled={files.isUploading} onclick={() => fileInput?.click()}>
			<UploadIcon class="size-4" />
			{files.isUploading ? text.uploading : text.upload}
		</Button>
		<input bind:this={fileInput} type="file" multiple class="hidden" onchange={onFilePick} />
	</div>

	{#if files.errorMessage}
		<p class="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
			{files.errorMessage}
		</p>
	{/if}

	<section
		aria-label={text.title}
		class="rounded-lg border transition-colors {isDragging ? 'border-primary bg-primary/5' : ''}"
		ondragover={onDragOver}
		ondragleave={onDragLeave}
		ondrop={onDrop}
	>
		{#if files.entries.length === 0}
			<p class="px-4 py-10 text-center text-sm text-muted-foreground">
				{files.isLoading ? '' : text.empty}
			</p>
		{:else}
			<ul class="divide-y">
				{#each files.entries as entry (entry.agentPath)}
					<li class="flex items-center gap-3 px-4 py-2.5 hover:bg-accent/50">
						{#if entry.isDirectory}
							<button
								type="button"
								class="flex min-w-0 flex-1 items-center gap-3 text-left"
								onclick={() => files.navigateTo(entry.agentPath)}
							>
								<FolderIcon class="size-4 shrink-0 text-muted-foreground" />
								<span class="truncate text-sm font-medium">{entry.name}</span>
							</button>
							<span class="shrink-0 text-xs text-muted-foreground">{text.folder}</span>
						{:else}
							<div class="flex min-w-0 flex-1 items-center gap-3">
								<FileIcon class="size-4 shrink-0 text-muted-foreground" />
								<span class="truncate text-sm">{entry.name}</span>
							</div>
							<span class="shrink-0 text-xs text-muted-foreground">{formatFileSize(entry.size)}</span>
							<a
								class="shrink-0 text-muted-foreground hover:text-foreground"
								href={workspaceDownloadURL(entry.agentPath)}
								aria-label={text.download}
								download
							>
								<DownloadIcon class="size-4" />
							</a>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}

		<p class="border-t px-4 py-2 text-center text-xs text-muted-foreground">{text.dropHint}</p>
	</section>
</div>

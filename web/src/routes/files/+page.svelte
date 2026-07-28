<script lang="ts">
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import * as FileDropZone from '$lib/components/ui/file-drop-zone';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import FileDetail from './file-detail.svelte';
	import { getFilesState } from './files-context.svelte';
	import { fileVisual, formatFileSize, formatModified } from './files-view';
	import { filesText } from './text';

	const text = createPageText(filesText);
	const files = getFilesState();
	const isDesktop = new MediaQuery('min-width: 1024px');

	let isDraggingOver = $state(false);

	const sortedEntries = $derived(
		[...files.currentEntries].sort((first, second) => {
			if (first.isDirectory !== second.isDirectory) return first.isDirectory ? -1 : 1;
			return first.name.localeCompare(second.name);
		})
	);
	const isDirectoryLoading = $derived(files.isLoading || files.isLoadingPath(files.currentPath));
	const isDockedPreviewOpen = $derived(files.selectedFile !== null && isDesktop.current);
	const isSheetPreviewOpen = $derived(files.selectedFile !== null && !isDesktop.current);

	function handleDrop(event: DragEvent) {
		event.preventDefault();
		isDraggingOver = false;
		const droppedFiles = Array.from(event.dataTransfer?.files ?? []);
		if (droppedFiles.length > 0) void files.upload(droppedFiles);
	}

	function handleDragLeave(event: DragEvent) {
		const nextTarget = event.relatedTarget;
		const region = event.currentTarget;
		if (nextTarget instanceof Node && region instanceof Node && region.contains(nextTarget)) return;
		isDraggingOver = false;
	}
</script>

<div class="flex flex-col gap-5">
	{#if files.errorMessage}
		<div
			class="border-destructive/40 bg-destructive/5 flex items-center justify-between gap-3 rounded-lg border px-4 py-3"
		>
			<span class="text-destructive text-sm">{files.errorMessage}</span>
			<Button variant="outline" size="sm" onclick={() => files.reload()}>{text.retry}</Button>
		</div>
	{/if}

	<FileDropZone.Root onUpload={(uploaded) => files.upload(uploaded)} disabled={files.isUploading}>
		<div class="flex flex-col gap-4">
			<div class="flex items-end justify-between gap-3">
				{#if files.currentRoot}
					<Breadcrumb.Root>
						<Breadcrumb.List>
							{#each files.breadcrumbs as crumb, index (crumb.path)}
								{#if index > 0}
									<Breadcrumb.Separator />
								{/if}
								<Breadcrumb.Item>
									{#if index === files.breadcrumbs.length - 1}
										<Breadcrumb.Page class="max-w-[14rem] truncate font-semibold">
											{crumb.label}
										</Breadcrumb.Page>
									{:else}
										<Breadcrumb.Link
											class="max-w-[12rem] cursor-pointer truncate"
											onclick={() => files.openDirectory(crumb.path)}
										>
											{crumb.label}
										</Breadcrumb.Link>
									{/if}
								</Breadcrumb.Item>
							{/each}
						</Breadcrumb.List>
					</Breadcrumb.Root>
				{/if}

				<div class="flex shrink-0 items-center gap-2">
					<TooltipIconButton label={text.refresh} variant="ghost" size="icon-sm" onclick={() => files.reload()}>
						<RefreshCwIcon class={files.isLoading ? 'animate-spin' : ''} />
					</TooltipIconButton>
					<FileDropZone.Trigger class={buttonVariants({ size: 'sm', variant: 'default' })}>
						{#if files.isUploading}
							<Spinner class="size-4" />
						{:else}
							<UploadIcon class="size-4" />
						{/if}
						{files.isUploading ? text.uploading : text.upload}
					</FileDropZone.Trigger>
				</div>
			</div>

			<div class="flex items-start gap-4">
				<div
					role="region"
					aria-label={text.title}
					class="bg-card relative min-w-0 flex-1 overflow-hidden rounded-xl border shadow-sm transition-colors {isDraggingOver
						? 'border-primary ring-primary/30 ring-2'
						: ''}"
					ondragenter={() => (isDraggingOver = true)}
					ondragover={(event) => event.preventDefault()}
					ondragleave={handleDragLeave}
					ondrop={handleDrop}
				>
					<div
						class="text-muted-foreground bg-muted/40 grid grid-cols-[1fr_5rem_7rem] gap-3 border-b px-4 py-2.5 text-xs font-medium"
					>
						<span>{text.name}</span>
						<span class="text-right">{text.size}</span>
						<span class="text-right">{text.modified}</span>
					</div>

					{#if isDirectoryLoading}
						<div class="flex items-center justify-center py-16">
							<Spinner class="text-muted-foreground" />
						</div>
					{:else if sortedEntries.length === 0}
						<div class="text-muted-foreground flex flex-col items-center gap-3 py-16 text-center">
							<div class="bg-muted flex size-12 items-center justify-center rounded-full">
								<FolderOpenIcon class="size-6" />
							</div>
							<p class="text-sm">{text.empty}</p>
						</div>
					{:else}
						{#each sortedEntries as entry (entry.agentPath)}
							{@const visual = fileVisual(entry.name)}
							{@const FileTypeIcon = visual.icon}
							{@const isSelected = files.selectedFile?.agentPath === entry.agentPath}
							<button
								type="button"
								class="hover:bg-accent/60 grid w-full grid-cols-[1fr_5rem_7rem] items-center gap-3 border-b px-4 py-2.5 text-left transition-colors last:border-b-0 {isSelected
									? 'bg-accent'
									: ''}"
								onclick={() =>
									entry.isDirectory ? files.openDirectory(entry.agentPath) : files.selectFile(entry)}
							>
								<span class="flex min-w-0 items-center gap-2.5">
									{#if entry.isDirectory}
										<FolderIcon class="size-4 shrink-0 fill-amber-200 text-amber-500" />
									{:else}
										<FileTypeIcon class="size-4 shrink-0 {visual.colorClass}" />
									{/if}
									<span class="truncate text-sm">{entry.name}</span>
								</span>
								<span
									class="text-muted-foreground flex items-center justify-end text-xs tabular-nums"
								>
									{#if entry.isDirectory}
										<ChevronRightIcon class="size-4" />
									{:else}
										{formatFileSize(entry.size)}
									{/if}
								</span>
								<span class="text-muted-foreground text-right text-xs">
									{formatModified(entry.modifiedAt, currentLocale.value)}
								</span>
							</button>
						{/each}
					{/if}

					{#if isDraggingOver}
						<div
							class="bg-primary/5 text-primary pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-2 backdrop-blur-[1px]"
						>
							<UploadIcon class="size-7" />
							<span class="text-sm font-medium">{text.dropHint}</span>
						</div>
					{/if}
				</div>

				{#if isDockedPreviewOpen && files.selectedFile}
					<aside
						class="bg-card sticky top-0 flex h-[calc(100vh-7rem)] w-[26rem] shrink-0 flex-col overflow-hidden rounded-xl border shadow-sm"
					>
						<FileDetail file={files.selectedFile} onClose={() => files.clearSelection()} />
					</aside>
				{/if}
			</div>
		</div>
	</FileDropZone.Root>
</div>

<Sheet.Root
	open={isSheetPreviewOpen}
	onOpenChange={(open) => {
		if (!open) files.clearSelection();
	}}
>
	<Sheet.Content side="right" class="w-full gap-0 p-0 sm:max-w-md">
		{#if files.selectedFile}
			<FileDetail file={files.selectedFile} />
		{/if}
	</Sheet.Content>
</Sheet.Root>

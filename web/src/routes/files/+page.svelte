<script lang="ts">
	import { onMount } from 'svelte';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import * as FileDropZone from '$lib/components/ui/file-drop-zone';
	import * as Select from '$lib/components/ui/select';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import FileBrowserList, { type FileBrowserEntry } from '$lib/components/file-browser-list.svelte';
	import FileBrowserPreview from '$lib/components/file-browser-preview.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import FileDetail from './file-detail.svelte';
	import FilesSidebar from './files-sidebar.svelte';
	import { getFilesState } from './files-context.svelte';
	import { formatFileSize, formatModified } from '$lib/files/view';
	import { filesText } from './text';

	const text = createPageText(filesText);
	const files = getFilesState();
	let isDraggingOver = $state(false);
	const entries = $derived(
		[...files.currentEntries]
			.sort(
				(first, second) =>
					Number(second.isDirectory) - Number(first.isDirectory) ||
					first.name.localeCompare(second.name)
			)
			.map((entry) => ({
				id: entry.agentPath,
				name: entry.name,
				isDirectory: entry.isDirectory,
				secondary: formatFileSize(entry.size),
				date: formatModified(entry.modifiedAt, currentLocale.value)
			}))
	);

	function openEntry(entry: FileBrowserEntry) {
		const selected = files.currentEntries.find((candidate) => candidate.agentPath === entry.id);
		if (!selected) return;
		if (selected.isDirectory) void files.openDirectory(selected.agentPath);
		else files.selectFile(selected);
	}

	onMount(() => {
		if (files.roots.length === 0) void files.loadRoots();
	});

	function handleDrop(event: DragEvent) {
		event.preventDefault();
		isDraggingOver = false;
		if (files.isUploading) return;
		const droppedFiles = Array.from(event.dataTransfer?.files ?? []);
		if (droppedFiles.length > 0) void files.upload(droppedFiles);
	}

	function handleDragLeave(event: DragEvent) {
		if (
			event.relatedTarget instanceof Node &&
			event.currentTarget instanceof Node &&
			event.currentTarget.contains(event.relatedTarget)
		)
			return;
		isDraggingOver = false;
	}
</script>

<div class="flex h-full min-h-0">
	<FilesSidebar />
	<div class="min-w-0 flex-1 overflow-auto p-3 sm:p-6">
		<div class="mb-4 grid gap-1.5 md:hidden">
			<label for="files-root" class="text-sm font-medium">{text.roots}</label>
			<Select.Root type="single" value={files.currentRoot?.id ?? ''} onValueChange={(id) => {
				const root = files.roots.find((root) => root.id === id);
				if (root) void files.openRoot(root);
			}}>
				<Select.Trigger id="files-root" class="w-full min-w-0"><span class="min-w-0 truncate">{files.currentRoot?.label ?? text.roots}</span></Select.Trigger>
				<Select.Content><Select.Group>
					{#each files.roots as root (root.id)}
						<Select.Item value={root.id} label={root.label}><span class="min-w-0 whitespace-normal break-words">{root.label}</span></Select.Item>
					{/each}
				</Select.Group></Select.Content>
			</Select.Root>
		</div>
		{#if files.errorMessage}
			<div role="alert" class="mb-4 flex items-center justify-between gap-3">
				<span class="text-destructive text-sm">{files.errorMessage}</span>
				<Button variant="outline" size="sm" onclick={() => files.reload()}>{text.retry}</Button>
			</div>
		{/if}
		<FileDropZone.Root onUpload={(uploaded) => files.upload(uploaded)} disabled={files.isUploading}>
			<div class="flex flex-col gap-4">
				<div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
					<Breadcrumb.Root class="min-w-0 max-w-full overflow-x-auto">
						<Breadcrumb.List class="w-max flex-nowrap">
							{#if files.isLoading && files.breadcrumbs.length === 0}
								<Breadcrumb.Item aria-hidden="true" class="h-5"><Skeleton class="h-4 w-24" /></Breadcrumb.Item>
							{/if}
							{#each files.breadcrumbs as crumb, index (crumb.path)}
								{#if index > 0}<Breadcrumb.Separator />{/if}
								<Breadcrumb.Item>
									{#if index === files.breadcrumbs.length - 1}<Breadcrumb.Page
											>{crumb.label}</Breadcrumb.Page
										>
									{:else}<Breadcrumb.Link onclick={() => files.openDirectory(crumb.path)}
											>{crumb.label}</Breadcrumb.Link
										>{/if}
								</Breadcrumb.Item>
							{/each}
						</Breadcrumb.List>
					</Breadcrumb.Root>
					<div class="flex shrink-0 items-center gap-2">
						<TooltipIconButton
							label={text.refresh}
							variant="ghost"
							size="icon-sm"
							onclick={() => files.reload()}><RefreshCwIcon /></TooltipIconButton
						>
						<FileDropZone.Trigger class={buttonVariants({ size: 'sm' })}>
							{#if files.isUploading}<Spinner />{:else}<UploadIcon />{/if}
							{files.isUploading ? `${text.uploading} ${Math.floor(files.uploadedFraction * 100)}%` : text.upload}
						</FileDropZone.Trigger>
					</div>
				</div>
				<div class="flex items-start gap-4">
					<div
						role="region"
						aria-label={text.title}
						class="relative min-w-0 flex-1"
						ondragenter={() => (isDraggingOver = true)}
						ondragover={(event) => event.preventDefault()}
						ondragleave={handleDragLeave}
						ondrop={handleDrop}
					>
						<FileBrowserList
							{entries}
							title={text.title}
							nameLabel={text.name}
							secondaryLabel={text.size}
							dateLabel={text.modified}
							emptyLabel={text.empty}
							selectedID={files.selectedFile?.agentPath}
								isLoading={files.isLoading || files.isLoadingPath(files.currentPath)}
								hasLoadError={Boolean(files.errorMessage)}
							onSelect={openEntry}
						/>
						{#if isDraggingOver}<div
								class="bg-primary/5 text-primary pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-primary backdrop-blur-[1px]"
							>
								<UploadIcon /><span class="text-sm font-medium">{text.dropHint}</span>
							</div>{/if}
					</div>
					<FileBrowserPreview
						isOpen={files.selectedFile !== null}
						title={files.selectedFile?.name ?? text.title}
						onClose={() => files.clearSelection()}
					>
						{#if files.selectedFile}<FileDetail
								file={files.selectedFile}
								onClose={() => files.clearSelection()}
							/>{/if}
					</FileBrowserPreview>
				</div>
			</div>
		</FileDropZone.Root>
	</div>
</div>

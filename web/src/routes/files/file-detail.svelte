<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import XIcon from '@lucide/svelte/icons/x';
	import DocumentPreview from '$lib/files/preview/document-preview.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import {
		isWorkspaceOnTheCompanyComputer,
		workspaceDownloadURL,
		workspaceFileForReading,
		type WorkspaceEntry
	} from './files-api';
	import { fileVisual, formatFileSize, formatModified } from '$lib/files/view';
	import { filesText } from './text';

	let { file, onClose }: { file: WorkspaceEntry; onClose?: () => void } = $props();

	const text = createPageText(filesText);
	const visual = $derived(fileVisual(file.name));
	const FileTypeIcon = $derived(visual.icon);
	const isOnTheCompanyComputer = isWorkspaceOnTheCompanyComputer();
	const deviceDownloadURL = $derived(workspaceDownloadURL(file.agentPath));
	let isPreparingDownload = $state(false);
	let preparedFraction = $state<number | null>(null);
	let downloadFailure = $state('');

	$effect(() => {
		void file;
		downloadFailure = '';
	});

	async function readableAddressOf(entry: WorkspaceEntry): Promise<string> {
		if (!isOnTheCompanyComputer) return workspaceDownloadURL(entry.agentPath);
		return workspaceFileForReading(entry, 'preview');
	}

	async function downloadFromTheCompanyComputer() {
		const entry = file;
		isPreparingDownload = true;
		preparedFraction = null;
		downloadFailure = '';
		try {
			const address = await workspaceFileForReading(entry, 'download', (copiedBytes, totalBytes) => {
				preparedFraction = totalBytes > 0 ? copiedBytes / totalBytes : null;
			});
			const anchor = document.createElement('a');
			anchor.href = address;
			anchor.download = entry.name;
			anchor.click();
		} catch (failure) {
			downloadFailure = failure instanceof Error && failure.message ? failure.message : text.downloadFailed;
		} finally {
			isPreparingDownload = false;
			preparedFraction = null;
		}
	}

	const preparingLabel = $derived(
		preparedFraction === null ? text.preparing : `${text.preparing} ${Math.floor(preparedFraction * 100)}%`
	);
</script>

<div class="flex h-full min-h-0 flex-col">
	<div class="flex items-start gap-3 border-b p-4 {onClose ? '' : 'pr-14'}">
		<div class="bg-muted flex size-11 shrink-0 items-center justify-center rounded-lg">
			<FileTypeIcon class="size-6 {visual.colorClass}" />
		</div>
		<div class="min-w-0 flex-1">
			<h2 class="break-all text-base leading-tight font-semibold">{file.name}</h2>
			<p class="text-muted-foreground mt-1 flex items-center gap-1.5 text-xs">
				<span class="tabular-nums">{formatFileSize(file.size)}</span>
				<span aria-hidden="true">·</span>
				<span>{formatModified(file.modifiedAt, currentLocale.value)}</span>
			</p>
		</div>
		{#if onClose}
			<Button variant="ghost" size="icon-sm" aria-label={text.close} onclick={onClose}>
				<XIcon />
			</Button>
		{/if}
	</div>

	<div class="bg-muted/40 min-h-0 flex-1">
		{#key file}<DocumentPreview fileName={file.name} size={file.size} readURL={() => readableAddressOf(file)} />{/key}
	</div>

	<div class="flex flex-col gap-2 border-t p-4 max-sm:pb-[max(1rem,env(safe-area-inset-bottom))]">
		{#if isOnTheCompanyComputer}
			<Button onclick={downloadFromTheCompanyComputer} disabled={isPreparingDownload} class="w-full">
				<DownloadIcon />
				{isPreparingDownload ? preparingLabel : downloadFailure ? text.retry : text.download}
			</Button>
			{#if downloadFailure}
				<p role="alert" class="text-destructive text-sm">{downloadFailure}</p>
			{/if}
		{:else}
			<Button href={deviceDownloadURL} download={file.name} class="w-full">
				<DownloadIcon />
				{text.download}
			</Button>
		{/if}
	</div>
</div>

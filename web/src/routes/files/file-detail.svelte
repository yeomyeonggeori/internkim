<script lang="ts">
	import * as Code from '$lib/components/ui/code';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import XIcon from '@lucide/svelte/icons/x';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import {
		isWorkspaceOnTheCompanyComputer,
		workspaceDownloadURL,
		workspaceFileForReading,
		type WorkspaceEntry
	} from './files-api';
	import {
		codeLanguageForFile,
		fileVisual,
		formatFileSize,
		formatModified,
		parseDelimitedText
	} from '$lib/files/view';
	import { filesText } from './text';

	let { file, onClose }: { file: WorkspaceEntry; onClose?: () => void } = $props();

	const text = createPageText(filesText);
	const visual = $derived(fileVisual(file.name));
	const FileTypeIcon = $derived(visual.icon);
	const imagePattern = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/i;
	const tabularPattern = /\.(csv|tsv)$/i;
	const maxPreviewBytes = 2 * 1024 * 1024;
	const maxImagePreviewBytes = 20 * 1024 * 1024;
	const isOnTheCompanyComputer = isWorkspaceOnTheCompanyComputer();

	const isImage = $derived(imagePattern.test(file.name));
	const isTabular = $derived(tabularPattern.test(file.name));
	const deviceDownloadURL = $derived(workspaceDownloadURL(file.agentPath));
	const previewLanguage = $derived(codeLanguageForFile(file.name));
	const isTextFile = $derived(!isImage && previewLanguage !== null);
	const canPreviewText = $derived(isTextFile && file.size <= maxPreviewBytes);
	const isTextFileTooLarge = $derived(isTextFile && file.size > maxPreviewBytes);
	const canPreviewImage = $derived(isImage && (!isOnTheCompanyComputer || file.size <= maxImagePreviewBytes));
	let previewContent = $state('');
	let previewURL = $state('');
	let isPreviewLoading = $state(false);
	let hasPreviewError = $state(false);
	let isPreparingDownload = $state(false);
	let preparedFraction = $state<number | null>(null);
	let downloadFailure = $state('');

	const imageSource = $derived(isOnTheCompanyComputer ? previewURL : deviceDownloadURL);

	const tableDelimiter = $derived(file.name.toLowerCase().endsWith('.tsv') ? '\t' : ',');
	const tableRows = $derived(isTabular ? parseDelimitedText(previewContent, tableDelimiter) : []);

	$effect(() => {
		downloadFailure = '';
		previewContent = '';
		previewURL = '';
		if (!canPreviewText && !(isOnTheCompanyComputer && canPreviewImage)) return;
		void loadPreview(file);
	});

	async function readableAddressOf(entry: WorkspaceEntry): Promise<string> {
		if (!isOnTheCompanyComputer) return workspaceDownloadURL(entry.agentPath);
		return workspaceFileForReading(entry, 'preview');
	}

	async function loadPreview(entry: WorkspaceEntry) {
		isPreviewLoading = true;
		hasPreviewError = false;
		try {
			const address = await readableAddressOf(entry);
			if (entry !== file) return;
			if (isImage) {
				previewURL = address;
				return;
			}
			const response = await fetch(address, isOnTheCompanyComputer ? {} : { credentials: 'include' });
			if (!response.ok) throw new Error(`preview returned ${response.status}`);
			const content = await response.text();
			if (entry === file) previewContent = content;
		} catch {
			if (entry === file) hasPreviewError = true;
		} finally {
			if (entry === file) isPreviewLoading = false;
		}
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

	<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-auto p-4">
		{#if canPreviewImage && imageSource}
		<img
			src={imageSource}
			alt={file.name}
			class="bg-muted max-h-80 w-full rounded-md border object-contain"
		/>
	{:else if canPreviewImage && isPreviewLoading}
		<p class="text-muted-foreground text-sm">{text.previewLoading}</p>
	{:else if canPreviewImage && hasPreviewError}
		<p class="text-destructive text-sm">{text.previewError}</p>
	{:else if canPreviewText}
		{#if isPreviewLoading}
			<p class="text-muted-foreground text-sm">{text.previewLoading}</p>
		{:else if hasPreviewError}
			<p class="text-destructive text-sm">{text.previewError}</p>
		{:else if isTabular}
			<div class="overflow-auto rounded-lg border">
				<table class="w-full border-collapse text-sm">
					<tbody>
						{#each tableRows as row, rowIndex (rowIndex)}
							<tr class="border-b last:border-b-0">
								{#each row as cell, cellIndex (cellIndex)}
									{#if rowIndex === 0}
										<th class="bg-muted/50 text-muted-foreground border-r px-3 py-2 text-left font-medium last:border-r-0">
											{cell}
										</th>
									{:else}
										<td class="border-r px-3 py-1.5 tabular-nums last:border-r-0">{cell}</td>
									{/if}
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<Code.Root code={previewContent} lang={previewLanguage ?? 'markdown'} class="text-xs" />
		{/if}
		{:else}
			<div class="text-muted-foreground bg-muted/40 flex flex-col items-center gap-2 rounded-lg border border-dashed py-10 text-center">
				<FileIcon class="size-6" />
				<p class="px-6 text-sm">{isTextFileTooLarge ? text.previewTooLarge : text.previewUnavailable}</p>
			</div>
		{/if}
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

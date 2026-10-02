<script lang="ts">
	import * as Code from '$lib/components/ui/code';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import XIcon from '@lucide/svelte/icons/x';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { workspaceDownloadURL, type WorkspaceEntry } from './files-api';
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

	const isImage = $derived(imagePattern.test(file.name));
	const isTabular = $derived(tabularPattern.test(file.name));
	const downloadURL = $derived(workspaceDownloadURL(file.agentPath));
	const previewLanguage = $derived(codeLanguageForFile(file.name));
	const isTextFile = $derived(!isImage && previewLanguage !== null);
	const canPreviewText = $derived(isTextFile && file.size <= maxPreviewBytes);
	const isTextFileTooLarge = $derived(isTextFile && file.size > maxPreviewBytes);

	let previewContent = $state('');
	let isPreviewLoading = $state(false);
	let hasPreviewError = $state(false);

	const tableDelimiter = $derived(file.name.toLowerCase().endsWith('.tsv') ? '\t' : ',');
	const tableRows = $derived(isTabular ? parseDelimitedText(previewContent, tableDelimiter) : []);

	$effect(() => {
		if (!canPreviewText) {
			previewContent = '';
			return;
		}
		void loadTextPreview(downloadURL);
	});

	async function loadTextPreview(url: string) {
		isPreviewLoading = true;
		hasPreviewError = false;
		previewContent = '';
		try {
			const response = await fetch(url, { credentials: 'include' });
			if (!response.ok) throw new Error(`preview returned ${response.status}`);
			previewContent = await response.text();
		} catch {
			hasPreviewError = true;
		} finally {
			isPreviewLoading = false;
		}
	}
</script>

<div class="flex h-full min-h-0 flex-col">
	<div class="flex items-start gap-3 border-b p-4">
		<div class="bg-muted flex size-11 shrink-0 items-center justify-center rounded-lg">
			<FileTypeIcon class="size-6 {visual.colorClass}" />
		</div>
		<div class="min-w-0 flex-1">
			<h2 class="truncate text-base leading-tight font-semibold">{file.name}</h2>
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
		{#if isImage}
		<img
			src={downloadURL}
			alt={file.name}
			class="bg-muted max-h-80 w-full rounded-md border object-contain"
		/>
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

	<div class="border-t p-4">
		<Button href={downloadURL} download={file.name} class="w-full">
			<DownloadIcon />
			{text.download}
		</Button>
	</div>
</div>

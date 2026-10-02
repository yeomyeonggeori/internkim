<script lang="ts">
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import XIcon from '@lucide/svelte/icons/x';
	import { invokeTool } from '$lib/public-api-call';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { dataRoomText } from '$lib/data-room/text';
	import { documentFileName, type DataRoomDocument } from '$lib/data-room/browser';
	import { fileVisual } from '$lib/files/view';

	let {
		document,
		category,
		onClose
	}: { document: DataRoomDocument; category: string; onClose: () => void } = $props();
	const text = createPageText(dataRoomText);
	const visual = $derived(fileVisual(documentFileName(document)));
	const FileTypeIcon = $derived(visual.icon);
	let isDownloading = $state(false);
	let errorMessage = $state('');
	const metadata = $derived(
		[
			[text.category, category],
			[text.date, document.date],
			[text.documentNumber, document.documentNumber],
			[text.counterpart, document.counterpart],
			[text.status, document.status]
		].filter(([, value]) => value)
	);

	async function download() {
		const documentID = document.documentID;
		const fileName = documentFileName(document);
		isDownloading = true;
		errorMessage = '';
		try {
			const answer = await invokeTool('company_document_download', { documentHint: documentID });
			const { downloadURL } = z.object({ downloadURL: z.string().url() }).parse(answer);
			const link = window.document.createElement('a');
			link.href = downloadURL;
			link.download = fileName;
			link.rel = 'noopener';
			link.click();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.loadFailed;
		} finally {
			isDownloading = false;
		}
	}
</script>

<div class="flex h-full min-h-0 flex-col">
	<div class="flex items-start gap-3 border-b p-4">
		<div class="bg-muted flex size-11 shrink-0 items-center justify-center rounded-lg">
			<FileTypeIcon class={visual.colorClass} />
		</div>
		<div class="min-w-0 flex-1">
			<h2 class="break-words text-base font-semibold">{document.title}</h2>
			<p class="text-muted-foreground mt-1 text-xs">{category}</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.close} onclick={onClose}
			><XIcon /></Button
		>
	</div>
	<div class="flex min-h-0 flex-1 flex-col gap-6 overflow-auto p-4">
		<section>
			<h3 class="mb-2 text-sm font-medium">{text.summary}</h3>
			<p class="text-muted-foreground whitespace-pre-wrap text-sm leading-relaxed">
				{document.summary || text.noSummary}
			</p>
		</section>
		<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 text-sm">
			{#each metadata as [label, value] (label)}<dt class="text-muted-foreground">{label}</dt>
				<dd class="break-words">{value}</dd>{/each}
		</dl>
		{#if document.tags.length}<p class="text-muted-foreground text-xs">
				{document.tags.join(' · ')}
			</p>{/if}
		{#if errorMessage}<p role="alert" class="text-destructive text-sm">{errorMessage}</p>{/if}
	</div>
	{#if document.storagePath}
		<div class="border-t p-4">
			<Button class="w-full" disabled={isDownloading} onclick={download}
				><DownloadIcon data-icon="inline-start" />{text.download}</Button
			>
		</div>
	{/if}
</div>

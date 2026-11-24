<script lang="ts">
	import DownloadIcon from '@lucide/svelte/icons/download';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import type { Attachment as ElementAttachment } from 'svelte/attachments';
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import type { AttachmentState } from '$lib/components/ui/attachment/index.js';
	import DocumentPreview from '$lib/files/preview/document-preview.svelte';
	import { cardAspectRatioOf, cardPreviewOf, previewKindOf } from '$lib/files/preview/kind';
	import SheetGridTable from '$lib/files/preview/sheet-grid-table.svelte';
	import { thumbnailOf, usualPageAspectRatio, type Thumbnail } from '$lib/files/preview/thumbnail';
	import { fileVisual } from '$lib/files/view';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attachmentSource } from '$lib/stores/attachment-source.svelte';
	import { downloadAttachment } from './attachment-download';
	import DownloadIndicator from '$lib/files/download-indicator.svelte';
	import { downloadProgress } from '$lib/files/download-progress.svelte';
	import { downloadText } from '$lib/i18n/download-text';
	import type { ChannelMessageAttachment } from './channel-api';
	import { formatAttachmentMeta, formatReceivedBytes, preparingLabel } from './channel-attachments';

	let {
		attachment,
		attachmentState,
		onOpen
	}: { attachment: ChannelMessageAttachment; attachmentState: AttachmentState; onOpen?: () => void } = $props();

	const text = createPageText(channelText);
	const downloadingText = createPageText(downloadText);
	const filename = $derived(attachment.filename ?? attachment.url);
	const visual = $derived(fileVisual(filename));
	const FileTypeIcon = $derived(visual.icon);
	const cardPreview = $derived(cardPreviewOf(filename, attachment.sizeBytes));
	const hasPreview = $derived(cardPreview !== 'none');
	const wantsThumbnail = $derived(cardPreview === 'thumbnail');
	let thumbnail = $state.raw<Thumbnail | null>(null);
	let hasThumbnailSettled = $state(false);
	let isSeen = $state(false);
	let documentAspectRatio = $state<number | undefined>(undefined);
	const isDocumentShown = $derived(cardPreview === 'document' && isSeen && attachmentState === 'done');
	const isAwaitingDocument = $derived(cardPreview === 'document' && !isSeen && attachmentState !== 'error');
	const isAwaitingThumbnail = $derived(wantsThumbnail && !hasThumbnailSettled && attachmentState !== 'error');
	const isIconShown = $derived(!thumbnail && !isDocumentShown && !isAwaitingThumbnail && !isAwaitingDocument && attachmentState !== 'error');
	const pageAspectRatio = $derived(pageAspectRatioOf(thumbnail));
	const description = $derived([formatAttachmentMeta(attachment), countOf(thumbnail)].filter(Boolean).join(' · '));
	const download = $derived(downloadProgress.stateOf(attachment.url));

	function receivingDescription(receivedBytes: number, totalBytes: number): string {
		const knownTotalBytes = totalBytes > 0 ? totalBytes : (attachment.sizeBytes ?? 0);
		if (receivedBytes <= 0 && knownTotalBytes <= 0) return downloadingText.receiving;
		return `${downloadingText.receiving} · ${formatReceivedBytes(receivedBytes, knownTotalBytes)}`;
	}

	function pageAspectRatioOf(shown: Thumbnail | null): number | undefined {
		if (shown?.kind === 'page') return shown.aspectRatio;
		if (wantsThumbnail && previewKindOf(filename) === 'pdf') return usualPageAspectRatio;
		return documentAspectRatio;
	}

	function countOf(shown: Thumbnail | null): string {
		if (shown?.kind === 'page' && shown.pageCount > 1) return text.pageCount.replace('{count}', String(shown.pageCount));
		if (shown?.kind === 'sheet' && shown.sheetCount > 1) return text.sheetCount.replace('{count}', String(shown.sheetCount));
		return '';
	}

	function drawThumbnail(source: string) {
		thumbnailOf(attachment.url, () => source, filename)
			.then(
				(drawn) => (thumbnail = drawn),
				(error: unknown) => console.warn('attachment thumbnail failed', error)
			)
			.finally(() => (hasThumbnailSettled = true));
	}

	function previewsWhenSeen(source: string): ElementAttachment<HTMLElement> {
		return (element) => {
			const observer = new IntersectionObserver(
				([entry]) => {
					if (!entry?.isIntersecting) return;
					observer.disconnect();
					isSeen = true;
					if (wantsThumbnail) drawThumbnail(source);
				},
				{ rootMargin: '400px 0px' }
			);
			observer.observe(element);
			return () => observer.disconnect();
		};
	}
</script>

<Attachment.Root orientation={hasPreview ? 'vertical' : 'horizontal'} size={hasPreview ? 'lg' : 'default'} state={attachmentState}>
	{#if onOpen && attachmentState === 'done'}
		<Attachment.Trigger aria-label={`${text.openAttachment}: ${filename}`} onclick={onOpen} />
	{/if}
	<Attachment.Media
		variant={thumbnail || isDocumentShown ? 'image' : 'icon'}
		class={isIconShown ? visual.tileClass : undefined}
		style={pageAspectRatio ? `aspect-ratio: ${pageAspectRatio}` : undefined}
		{@attach hasPreview && attachment.source ? previewsWhenSeen(attachment.source) : undefined}
	>
		{#if thumbnail?.kind === 'page'}
			<img src={thumbnail.address} alt="" class="h-full w-full bg-white object-cover object-top" />
		{:else if thumbnail?.kind === 'sheet'}
			<div aria-hidden="true" class="bg-background pointer-events-none absolute inset-0 overflow-hidden">
				<div style="zoom: 0.55"><SheetGridTable grid={thumbnail.grid} /></div>
			</div>
		{:else if isDocumentShown && attachment.source}
			{@const source = attachment.source}
			<div aria-hidden="true" inert class="bg-background pointer-events-none absolute inset-0 overflow-hidden text-left">
				<div class="h-[200%] w-[200%] origin-top-left scale-50">
					<DocumentPreview
						fileName={filename}
						size={attachment.sizeBytes}
						readURL={() => Promise.resolve(source)}
						isMiniature
						onAspectRatio={(aspectRatio) => (documentAspectRatio = cardAspectRatioOf(aspectRatio))}
					/>
				</div>
			</div>
		{:else if isAwaitingThumbnail || isAwaitingDocument}
			<Skeleton class="absolute inset-0 rounded-none" />
		{:else}
			<FileTypeIcon class={attachmentState === 'error' ? undefined : 'text-white'} />
		{/if}
	</Attachment.Media>
	<Attachment.Content>
		<Attachment.Title title={filename}>
			{#if !onOpen && attachment.source && attachmentState === 'done'}
				<a href={attachment.source} target="_blank" rel="noreferrer">{filename}</a>
			{:else}
				{filename}
			{/if}
		</Attachment.Title>
		{#if attachmentState === 'uploading'}
			<Attachment.Description>{text.attachmentUploading}</Attachment.Description>
		{:else if attachmentState === 'error'}
			<Attachment.Description title={attachmentSource.failure(attachment.url)}>{text.attachmentUnavailable}</Attachment.Description>
		{:else if attachmentState === 'processing'}
			<Attachment.Description>
				{preparingLabel(text.attachmentPreparing, attachmentSource.progress(attachment.url))}
			</Attachment.Description>
		{:else if download?.phase === 'running'}
			<Attachment.Description>
				<span class="tabular-nums">{receivingDescription(download.progress.receivedBytes, download.progress.totalBytes)}</span>
			</Attachment.Description>
		{:else if description}
			<Attachment.Description>{description}</Attachment.Description>
		{/if}
	</Attachment.Content>
	{#if attachmentState === 'error'}
		<Attachment.Actions>
			<Attachment.Action aria-label={text.retry} onclick={() => attachmentSource.retry(attachment.url)}>
				<RotateCwIcon />
			</Attachment.Action>
		</Attachment.Actions>
	{:else if attachment.source && attachmentState !== 'uploading'}
		{@const source = attachment.source}
		<Attachment.Actions>
			<Attachment.Action
				variant={hasPreview ? 'secondary' : 'ghost'}
				aria-label={text.downloadAttachment}
				onclick={() => downloadAttachment(source, filename, attachment.url)}
			>
				<DownloadIndicator identity={attachment.url} class="size-6">
					{#snippet idle()}
						<DownloadIcon />
					{/snippet}
				</DownloadIndicator>
			</Attachment.Action>
		</Attachment.Actions>
	{/if}
</Attachment.Root>

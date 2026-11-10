<script lang="ts">
	import DownloadIcon from '@lucide/svelte/icons/download';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import type { Attachment as ElementAttachment } from 'svelte/attachments';
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import type { AttachmentState } from '$lib/components/ui/attachment/index.js';
	import { previewKindOf } from '$lib/files/preview/kind';
	import SheetGridTable from '$lib/files/preview/sheet-grid-table.svelte';
	import { hasThumbnail, thumbnailOf, usualPageAspectRatio, type Thumbnail } from '$lib/files/preview/thumbnail';
	import { fileVisual } from '$lib/files/view';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attachmentSource } from '$lib/stores/attachment-source.svelte';
	import { startDownload } from './attachment-download';
	import type { ChannelMessageAttachment } from './channel-api';
	import { formatAttachmentMeta, preparingLabel } from './channel-attachments';

	let {
		attachment,
		attachmentState,
		onOpen
	}: { attachment: ChannelMessageAttachment; attachmentState: AttachmentState; onOpen?: () => void } = $props();

	const text = createPageText(channelText);
	const filename = $derived(attachment.filename ?? attachment.url);
	const visual = $derived(fileVisual(filename));
	const FileTypeIcon = $derived(visual.icon);
	const wantsThumbnail = $derived(hasThumbnail(filename, attachment.sizeBytes));
	let thumbnail = $state.raw<Thumbnail | null>(null);
	const pageAspectRatio = $derived(pageAspectRatioOf(thumbnail));
	const description = $derived([formatAttachmentMeta(attachment), countOf(thumbnail)].filter(Boolean).join(' · '));

	function pageAspectRatioOf(shown: Thumbnail | null): number | undefined {
		if (shown?.kind === 'page') return shown.aspectRatio;
		if (wantsThumbnail && previewKindOf(filename) === 'pdf') return usualPageAspectRatio;
		return undefined;
	}

	function countOf(shown: Thumbnail | null): string {
		if (shown?.kind === 'page' && shown.pageCount > 1) return text.pageCount.replace('{count}', String(shown.pageCount));
		if (shown?.kind === 'sheet' && shown.sheetCount > 1) return text.sheetCount.replace('{count}', String(shown.sheetCount));
		return '';
	}

	function drawsWhenSeen(source: string): ElementAttachment<HTMLElement> {
		return (element) => {
			const observer = new IntersectionObserver(
				([entry]) => {
					if (!entry?.isIntersecting) return;
					observer.disconnect();
					thumbnailOf(attachment.url, () => source, filename).then(
						(drawn) => (thumbnail = drawn),
						(error: unknown) => console.warn('attachment thumbnail failed', error)
					);
				},
				{ rootMargin: '400px 0px' }
			);
			observer.observe(element);
			return () => observer.disconnect();
		};
	}
</script>

<Attachment.Root orientation={wantsThumbnail ? 'vertical' : 'horizontal'} size={wantsThumbnail ? 'lg' : 'default'} state={attachmentState}>
	{#if onOpen && attachmentState === 'done'}
		<Attachment.Trigger aria-label={`${text.openAttachment}: ${filename}`} onclick={onOpen} />
	{/if}
	<Attachment.Media
		variant={thumbnail ? 'image' : 'icon'}
		style={pageAspectRatio ? `aspect-ratio: ${pageAspectRatio}` : undefined}
		{@attach wantsThumbnail && attachment.source ? drawsWhenSeen(attachment.source) : undefined}
	>
		{#if thumbnail?.kind === 'page'}
			<img src={thumbnail.address} alt="" class="h-full w-full bg-white object-cover object-top" />
		{:else if thumbnail?.kind === 'sheet'}
			<div aria-hidden="true" class="bg-background pointer-events-none absolute inset-0 overflow-hidden">
				<div style="zoom: 0.55"><SheetGridTable grid={thumbnail.grid} /></div>
			</div>
		{:else}
			<FileTypeIcon class={visual.colorClass} />
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
				variant={wantsThumbnail ? 'secondary' : 'ghost'}
				aria-label={text.downloadAttachment}
				onclick={() => startDownload(source, filename)}
			>
				<DownloadIcon />
			</Attachment.Action>
		</Attachment.Actions>
	{/if}
</Attachment.Root>

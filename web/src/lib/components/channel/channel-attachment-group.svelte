<script lang="ts">
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import type { AttachmentState } from '$lib/components/ui/attachment/index.js';
	import LoadingImage from '$lib/components/loading-image.svelte';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attachmentSource } from '$lib/stores/attachment-source.svelte';
	import FileIcon from '@lucide/svelte/icons/file';
	import ImageOffIcon from '@lucide/svelte/icons/image-off';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import type { ChannelMessageAttachment } from './channel-api';
	import { attachmentStateOf, formatAttachmentMeta, preparingLabel } from './channel-attachments';
	import { startDownload } from './attachment-download';

	let {
		attachments,
		openPicture
	}: {
		attachments: ChannelMessageAttachment[];
		openPicture: (source: string) => void;
	} = $props();

	const text = createPageText(channelText);
	const pictures = $derived(attachments.filter((attachment) => attachment.kind === 'image'));
	const files = $derived(attachments.filter((attachment) => attachment.kind !== 'image'));

	function stateOf(attachment: ChannelMessageAttachment): AttachmentState {
		if (attachment.isUploading) return 'uploading';
		return attachmentStateOf(attachment.source, attachmentSource.status(attachment.url));
	}
</script>

<div class="flex min-w-0 flex-col gap-2">
	{#if pictures.length > 0}
		<Attachment.Group class="relative w-fit max-w-full">
			{#each pictures as attachment (attachment.url)}
				{@const attachmentState = stateOf(attachment)}
				<Attachment.Root orientation="vertical" state={attachmentState}>
					<Attachment.Media variant="image">
						{#if attachmentState === 'uploading'}
							<img src={attachment.source} alt={attachment.filename ?? ''} class="aspect-square h-full w-full object-cover" />
							<Spinner role="status" aria-label={text.attachmentUploading} class="absolute" />
						{:else if attachment.source}
							{@const source = attachment.source}
							{@const notShown = attachmentSource.isNotShown(attachment.url)}
							<button
								type="button"
								class="block h-full w-full cursor-zoom-in"
								aria-label={notShown ? `${text.attachmentUnavailable} · ${text.retry}` : (attachment.filename ?? '이미지 크게 보기')}
								onclick={() => (notShown ? attachmentSource.retry(attachment.url) : openPicture(source))}
							>
								<LoadingImage
									messagePicture
									src={source}
									alt={attachment.filename ?? ''}
									onFailed={() => attachmentSource.renew(attachment.url)}
									loading="lazy"
									fill
									class="aspect-square h-full w-full"
									imageClass="object-cover"
								/>
							</button>
						{:else if attachmentState === 'error'}
							<button
								type="button"
								class="flex h-full w-full items-center justify-center"
								aria-label={`${text.attachmentUnavailable} · ${text.retry}`}
								title={attachmentSource.failure(attachment.url) || text.attachmentUnavailable}
								onclick={() => attachmentSource.retry(attachment.url)}
							>
								<ImageOffIcon />
								<RotateCwIcon class="absolute top-1.5 right-1.5 size-4" />
							</button>
						{:else}
							<Spinner
								role="status"
								aria-label={preparingLabel(text.attachmentPreparing, attachmentSource.progress(attachment.url))}
							/>
						{/if}
					</Attachment.Media>
				</Attachment.Root>
			{/each}
		</Attachment.Group>
	{/if}
	{#each files as attachment (attachment.url)}
		{@const attachmentState = stateOf(attachment)}
		<Attachment.Root state={attachmentState}>
			<Attachment.Media>
				<FileIcon />
			</Attachment.Media>
			<Attachment.Content>
				<Attachment.Title>
					{#if attachment.source && attachmentState !== 'uploading'}
						<a href={attachment.source} target="_blank" rel="noreferrer">
							{attachment.filename ?? attachment.url}
						</a>
					{:else}
						{attachment.filename ?? attachment.url}
					{/if}
				</Attachment.Title>
				{#if attachmentState === 'uploading'}
					<Attachment.Description>{text.attachmentUploading}</Attachment.Description>
				{:else if attachmentState === 'error'}
					<Attachment.Description title={attachmentSource.failure(attachment.url)}>
						{text.attachmentUnavailable}
					</Attachment.Description>
				{:else if attachmentState === 'processing'}
					<Attachment.Description>
						{preparingLabel(text.attachmentPreparing, attachmentSource.progress(attachment.url))}
					</Attachment.Description>
				{:else if formatAttachmentMeta(attachment)}
					<Attachment.Description>
						{formatAttachmentMeta(attachment)}
					</Attachment.Description>
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
						aria-label={text.downloadAttachment}
						onclick={() => startDownload(source, attachment.filename ?? '')}
					>
						<DownloadIcon />
					</Attachment.Action>
				</Attachment.Actions>
			{/if}
		</Attachment.Root>
	{/each}
</div>

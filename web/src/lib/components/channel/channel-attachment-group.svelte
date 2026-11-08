<script lang="ts">
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import type { AttachmentState } from '$lib/components/ui/attachment/index.js';
	import LoadingImage from '$lib/components/loading-image.svelte';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attachmentSource } from '$lib/stores/attachment-source.svelte';
	import ImageOffIcon from '@lucide/svelte/icons/image-off';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import type { ChannelMessageAttachment } from './channel-api';
	import { previewKindOf } from '$lib/files/preview/kind';
	import { attachmentStateOf, preparingLabel } from './channel-attachments';
	import ChannelFileAttachment from './channel-file-attachment.svelte';

	let {
		attachments,
		openSource
	}: {
		attachments: ChannelMessageAttachment[];
		openSource: (source: string) => void;
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
								onclick={() => (notShown ? attachmentSource.retry(attachment.url) : openSource(source))}
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
		{@const source = attachment.source}
		<ChannelFileAttachment
			{attachment}
			attachmentState={stateOf(attachment)}
			onOpen={source && previewKindOf(attachment.filename ?? '') !== 'none' ? () => openSource(source) : undefined}
		/>
	{/each}
</div>

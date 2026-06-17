<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import XIcon from '@lucide/svelte/icons/x';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { workspaceDownloadURL, type WorkspaceEntry } from './files-api';
	import { filesText } from './text';

	let { file, onClose }: { file: WorkspaceEntry; onClose: () => void } = $props();

	const text = createPageText(filesText);
	const imagePattern = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/i;

	const isImage = $derived(imagePattern.test(file.name));
	const downloadURL = $derived(workspaceDownloadURL(file.agentPath));
</script>

<Card.Root class="w-72 shrink-0">
	<Card.Header class="flex flex-row items-start justify-between gap-2 space-y-0">
		<Card.Title class="text-sm break-all">{file.name}</Card.Title>
		<Button variant="ghost" size="icon-sm" onclick={onClose} aria-label={text.close}>
			<XIcon />
		</Button>
	</Card.Header>
	<Card.Content class="flex flex-col gap-3">
		{#if isImage}
			<img
				src={downloadURL}
				alt={file.name}
				class="bg-muted max-h-64 w-full rounded-md border object-contain"
			/>
		{:else}
			<div class="bg-muted flex h-32 items-center justify-center rounded-md border">
				<FileIcon class="text-muted-foreground size-10" />
			</div>
		{/if}
		<Button href={downloadURL} download={file.name} class="w-full">
			<DownloadIcon />
			{text.download}
		</Button>
	</Card.Content>
</Card.Root>

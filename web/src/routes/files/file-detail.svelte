<script lang="ts">
	import * as Sheet from '$lib/components/ui/sheet';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { workspaceDownloadURL, type WorkspaceEntry } from './files-api';
	import { filesText } from './text';

	let { file }: { file: WorkspaceEntry } = $props();

	const text = createPageText(filesText);
	const imagePattern = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/i;

	const isImage = $derived(imagePattern.test(file.name));
	const downloadURL = $derived(workspaceDownloadURL(file.agentPath));
</script>

<Sheet.Header>
	<Sheet.Title class="break-all">{file.name}</Sheet.Title>
</Sheet.Header>
<div class="flex flex-col gap-4 px-4">
	{#if isImage}
		<img
			src={downloadURL}
			alt={file.name}
			class="bg-muted max-h-80 w-full rounded-md border object-contain"
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
</div>

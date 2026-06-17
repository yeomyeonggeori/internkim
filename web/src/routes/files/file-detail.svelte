<script lang="ts">
	import * as Code from '$lib/components/ui/code';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { workspaceDownloadURL, type WorkspaceEntry } from './files-api';
	import { codeLanguageForFile } from './files-view';
	import { filesText } from './text';

	let { file }: { file: WorkspaceEntry } = $props();

	const text = createPageText(filesText);
	const imagePattern = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/i;
	const maxPreviewBytes = 512 * 1024;

	const isImage = $derived(imagePattern.test(file.name));
	const downloadURL = $derived(workspaceDownloadURL(file.agentPath));
	const previewLanguage = $derived(codeLanguageForFile(file.name));
	const canPreviewText = $derived(!isImage && previewLanguage !== null && file.size <= maxPreviewBytes);

	let previewContent = $state('');
	let isPreviewLoading = $state(false);
	let hasPreviewError = $state(false);

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
	{:else if canPreviewText}
		{#if isPreviewLoading}
			<p class="text-muted-foreground text-sm">{text.previewLoading}</p>
		{:else if hasPreviewError}
			<p class="text-destructive text-sm">{text.previewError}</p>
		{:else}
			<Code.Root
				code={previewContent}
				lang={previewLanguage ?? 'markdown'}
				class="max-h-96 text-xs"
			/>
		{/if}
	{:else}
		<p class="text-muted-foreground flex items-center gap-2 text-sm">
			<FileIcon class="size-4" />
			{text.previewUnavailable}
		</p>
	{/if}
	<Button href={downloadURL} download={file.name} class="w-full">
		<DownloadIcon />
		{text.download}
	</Button>
</div>

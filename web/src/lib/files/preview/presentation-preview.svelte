<script lang="ts">
	import libraryURL from 'pptx-preview/dist/pptx-preview.umd.js?url';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { presentationFrameDocument, presentationFrameMessageSchema } from './sandboxed-frame';
	import { previewText } from './text';

	let { bytes, onFailure }: { bytes: ArrayBuffer; onFailure: (error: unknown) => void } = $props();

	const text = createPageText(previewText);
	const frameDocument = presentationFrameDocument(new URL(libraryURL, window.location.href).href);
	let frame = $state<HTMLIFrameElement | null>(null);
	let slideCount = $state<number | null>(null);

	function receive(event: MessageEvent) {
		if (!frame || event.source !== frame.contentWindow) return;
		const message = presentationFrameMessageSchema.safeParse(event.data);
		if (!message.success) return;
		if (message.data.type === 'ready') frame.contentWindow?.postMessage(bytes.slice(0), '*');
		if (message.data.type === 'rendered') slideCount = message.data.slideCount;
		if (message.data.type === 'failed') onFailure(new Error(message.data.message));
	}
</script>

<svelte:window onmessage={receive} />

<div class="relative flex h-full min-h-0 flex-col">
	{#if slideCount !== null}
		<p class="text-muted-foreground bg-background/80 border-b px-3 py-1.5 text-xs tabular-nums backdrop-blur">
			{slideCount} {text.slides}
		</p>
	{:else}
		<div class="absolute inset-0 flex items-center justify-center"><Spinner /></div>
	{/if}
	<iframe
		bind:this={frame}
		title={text.slides}
		sandbox="allow-scripts"
		srcdoc={frameDocument}
		class="min-h-0 w-full flex-1 border-0"
		class:invisible={slideCount === null}
	></iframe>
</div>

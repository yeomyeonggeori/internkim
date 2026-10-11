<script lang="ts">
	import { Spinner } from '$lib/components/ui/spinner';
	import { frameMessageSchema } from './sandboxed-frame';

	let {
		bytes,
		frameDocument,
		countLabel,
		onFailure
	}: { bytes: ArrayBuffer; frameDocument: string; countLabel: string; onFailure: (error: unknown) => void } = $props();

	let frame = $state<HTMLIFrameElement | null>(null);
	let count = $state<number | null>(null);

	function receive(event: MessageEvent) {
		if (!frame || event.source !== frame.contentWindow) return;
		const message = frameMessageSchema.safeParse(event.data);
		if (!message.success) return;
		if (message.data.type === 'ready') frame.contentWindow?.postMessage(bytes.slice(0), '*');
		if (message.data.type === 'rendered') count = message.data.count;
		if (message.data.type === 'failed') onFailure(new Error(message.data.message));
	}
</script>

<svelte:window onmessage={receive} />

<div class="relative flex h-full min-h-0 flex-col">
	{#if count !== null}
		<p class="text-muted-foreground bg-background/80 border-b px-3 py-1.5 text-xs tabular-nums backdrop-blur">
			{count} {countLabel}
		</p>
	{:else}
		<div class="absolute inset-0 flex items-center justify-center"><Spinner /></div>
	{/if}
	<iframe
		bind:this={frame}
		title={countLabel}
		sandbox="allow-scripts"
		srcdoc={frameDocument}
		class="min-h-0 w-full flex-1 border-0"
		class:invisible={count === null}
	></iframe>
</div>

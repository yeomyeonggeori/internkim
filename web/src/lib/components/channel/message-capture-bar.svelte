<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import XIcon from '@lucide/svelte/icons/x';
	import { toast } from 'svelte-sonner';
	import { copyCaptureImage, messagesAsPNG, saveCaptureImage } from './message-capture-image';
	import type { CopyOutcome } from './message-copy';

	let {
		scroller,
		chosenIDs,
		onClearChoice,
		onCancel,
		onDone
	}: {
		scroller: HTMLElement | null;
		chosenIDs: string[];
		onClearChoice: () => void;
		onCancel: () => void;
		onDone: () => void;
	} = $props();

	const text = createPageText(channelText);
	const count = $derived(chosenIDs.length);
	const hasChoice = $derived(count > 0);
	let isWorking = $state(false);

	async function deliver(
		handOver: (picture: Promise<Blob>) => Promise<CopyOutcome>,
		successMessage: string
	): Promise<void> {
		if (!scroller || !hasChoice || isWorking) return;
		isWorking = true;
		const picture = messagesAsPNG(scroller, chosenIDs);
		const outcome = await handOver(picture).finally(() => (isWorking = false));
		if (outcome === 'failure') {
			toast.error(text.captureFailed);
			return;
		}
		toast.success(successMessage);
		onDone();
	}
</script>

<div role="toolbar" aria-label={text.captureConversation} class="bg-background grid grid-rows-2 border-t px-4 py-2" data-capture-bar>
	<div class="flex min-h-9 items-center justify-between gap-2 text-sm">
		{#if hasChoice}
			<span>{text.captureChosen} <span class="text-muted-foreground">{text.captureCount.replace('{count}', String(count))}</span></span>
			<Button variant="ghost" size="sm" onclick={onClearChoice}>{text.captureClearChoice}</Button>
		{:else}
			<span class="text-muted-foreground">{text.captureHint}</span>
		{/if}
	</div>
	<div class="flex min-h-9 items-center justify-end gap-2">
		{#if isWorking}<Spinner class="size-4" />{/if}
		<Button variant="outline" size="sm" disabled={!hasChoice || isWorking} onclick={() => void deliver(copyCaptureImage, text.captureCopied)}>
			<CopyIcon data-icon="inline-start" />
			{text.captureCopy}
		</Button>
		<Button variant="outline" size="sm" disabled={!hasChoice || isWorking} onclick={() => void deliver(saveCaptureImage, text.captureSaved)}>
			<DownloadIcon data-icon="inline-start" />
			{text.captureSave}
		</Button>
		<Button variant="ghost" size="sm" onclick={onCancel}>
			<XIcon data-icon="inline-start" />
			{text.captureCancel}
		</Button>
	</div>
</div>

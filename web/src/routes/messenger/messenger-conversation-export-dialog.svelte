<script lang="ts">
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import PrinterIcon from '@lucide/svelte/icons/printer';
	import SheetIcon from '@lucide/svelte/icons/sheet';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import type { ChannelMessage } from '$lib/components/channel/channel-api';
	import { conversationExportText } from '$lib/i18n/conversation-export-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		conversationAsCSV,
		conversationAsPrintableHTML,
		conversationAsText,
		exportFilename,
		type ConversationDocument,
		type ExportFormat
	} from '$lib/messenger/conversation-export';
	import { printDocument, saveFile } from '$lib/messenger/conversation-export-file';
	import { collectWholeConversation } from '$lib/messenger/conversation-history';

	let {
		open = $bindable(false),
		conversation
	}: {
		open?: boolean;
		conversation: { id: string; name: string } | null;
	} = $props();

	const text = createPageText(conversationExportText);
	const previewMessageCount = 8;

	type Loading = { state: 'loading'; count: number } | { state: 'failed' } | { state: 'ready'; messages: ChannelMessage[] };

	let format = $state<ExportFormat>('text');
	let loading = $state<Loading>({ state: 'loading', count: 0 });
	let attempt = $state(0);

	$effect(() => {
		if (!open || !conversation) return;
		void attempt;
		const controller = new AbortController();
		loading = { state: 'loading', count: 0 };
		collectWholeConversation(conversation.id, (count) => (loading = { state: 'loading', count }), controller.signal)
			.then((messages) => (loading = { state: 'ready', messages }))
			.catch((failure: unknown) => {
				if (controller.signal.aborted) return;
				console.warn('the conversation could not be collected for export', failure);
				loading = { state: 'failed' };
			});
		return () => controller.abort();
	});

	function documentOf(messages: ChannelMessage[]): ConversationDocument {
		return {
			title: conversation?.name ?? '',
			savedAt: new Date().toISOString(),
			messages,
			labels: {
				savedAt: text.savedAt,
				attachment: text.attachment,
				date: text.columnDate,
				time: text.columnTime,
				sender: text.columnSender,
				content: text.columnContent,
				attachments: text.columnAttachments
			}
		};
	}

	const previewDocument = $derived(
		loading.state === 'ready' ? documentOf(loading.messages.slice(0, previewMessageCount)) : null
	);
	const hiddenCount = $derived(
		loading.state === 'ready' ? Math.max(0, loading.messages.length - previewMessageCount) : 0
	);

	function exportConversation() {
		if (loading.state !== 'ready') return;
		const whole = documentOf(loading.messages);
		if (format === 'pdf') {
			printDocument(conversationAsPrintableHTML(whole));
		} else if (format === 'csv') {
			saveFile(exportFilename(whole.title, whole.savedAt, 'csv'), conversationAsCSV(whole), 'text/csv;charset=utf-8');
		} else {
			saveFile(exportFilename(whole.title, whole.savedAt, 'txt'), conversationAsText(whole), 'text/plain;charset=utf-8');
		}
		open = false;
	}

	type FormatOption = { value: ExportFormat; label: string; hint: string; icon: typeof FileTextIcon };

	const formats: FormatOption[] = $derived([
		{ value: 'text', label: text.formatText, hint: text.formatTextHint, icon: FileTextIcon },
		{ value: 'csv', label: text.formatCSV, hint: text.formatCSVHint, icon: SheetIcon },
		{ value: 'pdf', label: text.formatPDF, hint: text.formatPDFHint, icon: PrinterIcon }
	]);

	function chooseFormat(value: string) {
		const chosen = formats.find((option) => option.value === value);
		if (chosen) format = chosen.value;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-3xl" closeLabel={text.cancel}>
		<Dialog.Header>
			<Dialog.Title>{text.exportTitle.replace('{name}', conversation?.name ?? '')}</Dialog.Title>
			<Dialog.Description>{text.exportDescription}</Dialog.Description>
		</Dialog.Header>
		<div class="grid min-h-0 gap-4 sm:grid-cols-[13rem_1fr]">
			<ToggleGroup.Root
				type="single"
				orientation="vertical"
				variant="outline"
				spacing={2}
				value={format}
				onValueChange={chooseFormat}
				aria-label={text.formatLabel}
				class="w-full flex-col items-stretch"
			>
				{#each formats as option (option.value)}
					<ToggleGroup.Item value={option.value} class="h-auto justify-start py-2 text-left">
						<option.icon />
						<span class="grid">
							<span>{option.label}</span>
							<span class="text-muted-foreground text-xs font-normal">{option.hint}</span>
						</span>
					</ToggleGroup.Item>
				{/each}
			</ToggleGroup.Root>
			<section aria-label={text.preview} class="bg-muted/40 h-72 min-w-0 overflow-auto rounded-lg border">
				{#if loading.state === 'loading'}
					<div class="text-muted-foreground flex h-full items-center justify-center gap-2" role="status">
						<Spinner />
						<span>{text.loadingMessages.replace('{count}', String(loading.count))}</span>
					</div>
				{:else if loading.state === 'failed'}
					<div class="flex h-full flex-col items-center justify-center gap-3">
						<p role="alert" class="text-destructive">{text.loadFailed}</p>
						<Button variant="outline" size="sm" onclick={() => (attempt += 1)}>{text.retry}</Button>
					</div>
				{:else if previewDocument && previewDocument.messages.length === 0}
					<p class="text-muted-foreground flex h-full items-center justify-center">{text.previewEmpty}</p>
				{:else if previewDocument && format === 'pdf'}
					<iframe
						title={text.preview}
						sandbox=""
						srcdoc={conversationAsPrintableHTML(previewDocument)}
						class="h-full w-full bg-white"
					></iframe>
				{:else if previewDocument}
					<pre class="p-3 font-mono text-xs whitespace-pre-wrap">{format === 'csv'
							? conversationAsCSV(previewDocument).replace('﻿', '')
							: conversationAsText(previewDocument)}{#if hiddenCount > 0}{text.previewMore.replace('{count}', String(hiddenCount))}{/if}</pre>
				{/if}
			</section>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>{text.cancel}</Button>
			<Button disabled={loading.state !== 'ready'} onclick={exportConversation}>{text.export}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

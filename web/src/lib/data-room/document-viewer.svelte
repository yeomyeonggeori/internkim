<script lang="ts">
	import { untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { Dialog } from 'bits-ui';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import InfoIcon from '@lucide/svelte/icons/info';
	import XIcon from '@lucide/svelte/icons/x';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import DocumentPreview from '$lib/files/preview/document-preview.svelte';
	import { fileVisual } from '$lib/files/view';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { dataRoomText } from './text';
	import { neighbourID, type ViewerDocument, type ViewerSource } from './viewer';

	let {
		documents,
		openID = $bindable(),
		readSource,
		onDownload,
		isDownloading = false
	}: {
		documents: ViewerDocument[];
		openID: string | null;
		readSource: (document: ViewerDocument) => Promise<ViewerSource | null>;
		onDownload?: (document: ViewerDocument) => void;
		isDownloading?: boolean;
	} = $props();

	const text = createPageText(dataRoomText);
	const openDocument = $derived(documents.find((document) => document.id === openID));
	const position = $derived(documents.findIndex((document) => document.id === openID));
	const visual = $derived(fileVisual(openDocument?.fileName ?? ''));
	const FileTypeIcon = $derived(visual.icon);
	const previousID = $derived(openID ? neighbourID(documents, openID, -1) : undefined);
	const nextID = $derived(openID ? neighbourID(documents, openID, 1) : undefined);
	let source = $state<ViewerSource | null | undefined>(undefined);
	let sourceFailure = $state('');
	const openFileName = $derived(openDocument?.fileName ?? null);
	let isDetailsOpen = $state(new MediaQuery('min-width: 1024px').current);

	$effect(() => {
		const requestedID = openID;
		const hasFile = openFileName !== null;
		source = undefined;
		sourceFailure = '';
		if (!requestedID || !hasFile) return;
		const requested = untrack(() => openDocument);
		if (!requested) return;
		untrack(() => readSource)(requested).then(
			(answer) => {
				if (requested.id === openID) source = answer;
			},
			(error: unknown) => {
				if (requested.id === openID) sourceFailure = error instanceof Error ? error.message : text.loadFailed;
			}
		);
	});

	function moveWith(event: KeyboardEvent) {
		if (!openDocument) return;
		if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) return;
		if (event.key === 'ArrowLeft' && previousID) openID = previousID;
		if (event.key === 'ArrowRight' && nextID) openID = nextID;
	}
</script>

<svelte:window onkeydown={moveWith} />

<Dialog.Root
	open={openDocument !== undefined}
	onOpenChange={(isOpen) => {
		if (!isOpen) openID = null;
	}}
>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-(--layer-overlay) bg-black/60" />
		<Dialog.Content
			onOpenAutoFocus={(event) => event.preventDefault()}
			class="bg-background fixed inset-0 z-(--layer-panel) flex flex-col overflow-hidden outline-none sm:inset-3 sm:rounded-xl sm:border sm:shadow-2xl"
		>
			{#if openDocument}
				<header class="flex items-center gap-3 border-b px-3 py-2 sm:px-4">
					<div class="bg-muted flex size-9 shrink-0 items-center justify-center rounded-md">
						<FileTypeIcon class="size-5 {visual.colorClass}" />
					</div>
					<div class="min-w-0 flex-1">
						<Dialog.Title class="truncate text-sm font-semibold">{openDocument.title}</Dialog.Title>
						<Dialog.Description class="text-muted-foreground truncate text-xs">
							{[openDocument.category, openDocument.date].filter(Boolean).join(' · ')}
						</Dialog.Description>
					</div>
					<div class="flex shrink-0 items-center gap-1">
						<TooltipIconButton
							label={text.previous}
							variant="ghost"
							size="icon-sm"
							disabled={!previousID}
							onclick={() => (openID = previousID ?? openID)}><ChevronLeftIcon /></TooltipIconButton
						>
						<span class="text-muted-foreground hidden w-14 text-center text-xs tabular-nums sm:inline"
							>{position + 1} / {documents.length}</span
						>
						<TooltipIconButton
							label={text.next}
							variant="ghost"
							size="icon-sm"
							disabled={!nextID}
							onclick={() => (openID = nextID ?? openID)}><ChevronRightIcon /></TooltipIconButton
						>
						<TooltipIconButton
							label={text.details}
							variant={isDetailsOpen ? 'secondary' : 'ghost'}
							size="icon-sm"
							onclick={() => (isDetailsOpen = !isDetailsOpen)}><InfoIcon /></TooltipIconButton
						>
						{#if onDownload && openDocument.fileName}
							<Button size="sm" variant="outline" disabled={isDownloading} onclick={() => openDocument && onDownload(openDocument)}
								>{#if isDownloading}<Spinner />{:else}<DownloadIcon data-icon="inline-start" />{/if}<span class="max-sm:sr-only"
									>{text.download}</span
								></Button
							>
						{/if}
						<Dialog.Close>
							{#snippet child({ props })}
								<Button {...props} variant="ghost" size="icon-sm" aria-label={text.close}><XIcon /></Button>
							{/snippet}
						</Dialog.Close>
					</div>
				</header>
				<div class="flex min-h-0 flex-1 flex-col lg:flex-row">
					<div class="bg-muted/50 flex min-w-0 flex-1 flex-col">
						{#if source?.isTextPreview}
							<p class="text-muted-foreground border-b bg-amber-50 px-4 py-2 text-xs dark:bg-amber-950/30">
								{text.textPreviewNotice}
							</p>
						{/if}
						<div class="min-h-0 flex-1">
							{#if !openDocument.fileName}
								<p class="text-muted-foreground flex h-full items-center justify-center p-6 text-sm">{text.noFile}</p>
							{:else if sourceFailure}
								<p role="alert" class="text-destructive flex h-full items-center justify-center p-6 text-sm">
									{sourceFailure}
								</p>
							{:else if source}
								{#key source}<DocumentPreview {source} />{/key}
							{:else if source === null}
								<p class="text-muted-foreground flex h-full items-center justify-center p-6 text-center text-sm">{text.noPreview}</p>
							{:else}
								<div class="flex h-full items-center justify-center"><Spinner /></div>
							{/if}
						</div>
					</div>
					{#if isDetailsOpen}
						<aside class="flex shrink-0 flex-col gap-6 overflow-auto p-5 max-lg:max-h-[40%] max-lg:border-t lg:w-80 lg:border-l">
							<section>
								<h3 class="mb-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">{text.summary}</h3>
								<p class="text-sm leading-relaxed whitespace-pre-wrap">{openDocument.summary || text.noSummary}</p>
							</section>
							{#if openDocument.details.length}
								<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2.5 text-sm">
									{#each openDocument.details as [label, value] (label)}
										<dt class="text-muted-foreground">{label}</dt>
										<dd class="break-words">{value}</dd>
									{/each}
								</dl>
							{/if}
						</aside>
					{/if}
				</div>
			{/if}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>

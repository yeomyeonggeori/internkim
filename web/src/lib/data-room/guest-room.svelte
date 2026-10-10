<script lang="ts">
	import LockKeyholeIcon from '@lucide/svelte/icons/lock-keyhole';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { Badge } from '$lib/components/ui/badge';
	import * as InputGroup from '$lib/components/ui/input-group';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { fileVisual } from '$lib/files/view';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import DocumentViewer from './document-viewer.svelte';
	import {
		documentsInFolder,
		guestFolders,
		matchingDocuments,
		sharedCategoryName,
		sharedFileName,
		type SharedDocument,
		type SharedRoom
	} from './guest-room';
	import { dataRoomText } from './text';
	import { previewSourceOf, type ViewerDocument } from './viewer';

	let {
		room,
		canDownload,
		expiresAt,
		signFile,
		onDownload,
		isDownloading = false,
		onRefresh
	}: {
		room: SharedRoom;
		canDownload?: boolean;
		expiresAt?: string;
		signFile: (documentID: string, derivedFileName?: string) => Promise<string>;
		onDownload: (documentID: string) => void;
		isDownloading?: boolean;
		onRefresh: () => void;
	} = $props();

	const text = createPageText(dataRoomText);
	let selectedCode = $state('');
	let query = $state('');
	let openID = $state<string | null>(null);
	const folders = $derived(guestFolders(room, currentLocale.value));
	const selectedFolder = $derived(folders.find((folder) => folder.code === selectedCode));
	const visibleDocuments = $derived(matchingDocuments(documentsInFolder(room, selectedCode), query));
	const viewerDocuments = $derived(visibleDocuments.map(viewerDocumentOf));
	const expiryLabel = $derived(
		expiresAt
			? new Date(expiresAt).toLocaleString(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US', {
					month: 'short',
					day: 'numeric',
					hour: '2-digit',
					minute: '2-digit'
				})
			: ''
	);

	function categoryLabel(code: string): string {
		const category = room.categories.find((candidate) => candidate.code === code);
		return category ? sharedCategoryName(category, currentLocale.value) : code;
	}

	function viewerDocumentOf(document: SharedDocument): ViewerDocument {
		const details: [string, string][] = [
			[text.category, categoryLabel(document.category_code)],
			[text.date, document.document_date ?? ''],
			[text.status, document.status ?? ''],
			[text.fileType, document.extension?.toUpperCase() ?? '']
		];
		return {
			id: document.id,
			title: document.title,
			fileName: sharedFileName(document),
			category: categoryLabel(document.category_code),
			date: document.document_date ?? '',
			summary: document.summary ?? '',
			details: details.filter(([, value]) => value)
		};
	}

	function openFolder(code: string) {
		selectedCode = code;
		openID = null;
	}
</script>

<Tooltip.Provider delayDuration={120}>
<div class="bg-muted/40 flex min-h-dvh flex-col">
	<header class="bg-background sticky top-0 z-10 border-b">
		<div class="mx-auto flex w-full max-w-7xl items-center gap-3 px-4 py-3">
			<div class="bg-primary text-primary-foreground flex size-9 shrink-0 items-center justify-center rounded-lg">
				<LockKeyholeIcon class="size-4" />
			</div>
			<div class="min-w-0 flex-1">
				<h1 class="truncate text-base font-semibold">{text.sharedRoom}</h1>
				<p class="text-muted-foreground truncate text-xs">{text.confidential}</p>
			</div>
			{#if canDownload !== undefined}
				<Badge variant={canDownload ? 'secondary' : 'outline'}>{canDownload ? text.downloadAllowed : text.viewOnly}</Badge>
			{/if}
			{#if expiryLabel}
				<p class="text-muted-foreground hidden text-xs sm:block">{text.accessUntil} {expiryLabel}</p>
			{/if}
			<TooltipIconButton label={text.refresh} variant="ghost" size="icon-sm" onclick={onRefresh}
				><RefreshCwIcon /></TooltipIconButton
			>
		</div>
	</header>

	<div class="mx-auto grid w-full max-w-7xl flex-1 content-start gap-6 px-4 py-6 md:grid-cols-[15rem_minmax(0,1fr)]">
		<nav aria-label={text.folders} class="grid content-start gap-0.5 max-md:hidden">
			<p class="text-muted-foreground px-2 pb-2 text-xs font-medium">{text.folders}</p>
			{#snippet folderButton(code: string, name: string, documentCount: number, isChild: boolean)}
				<button
					type="button"
					aria-current={selectedCode === code ? 'page' : undefined}
					class={cn(
						'hover:bg-background flex items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm',
						isChild && 'pl-6',
						selectedCode === code && 'bg-background font-medium shadow-sm ring-1 ring-black/5'
					)}
					onclick={() => openFolder(code)}
				>
					<span class="min-w-0 truncate">{name}</span>
					<span class="text-muted-foreground text-xs tabular-nums">{documentCount}</span>
				</button>
			{/snippet}
			{@render folderButton('', text.allDocuments, room.documents.length, false)}
			{#each folders as folder (folder.code)}
				{@render folderButton(folder.code, folder.name, folder.documentCount, folder.isChild)}
			{/each}
		</nav>

		<section class="grid min-w-0 content-start gap-4" aria-labelledby="guest-room-folder">
			<Select.Root type="single" value={selectedCode || 'all'} onValueChange={(value) => openFolder(value === 'all' ? '' : value)}>
				<Select.Trigger class="w-full md:hidden" aria-label={text.folders}
					><span class="truncate">{selectedFolder?.name ?? text.allDocuments}</span></Select.Trigger
				>
				<Select.Content>
					<Select.Item value="all">{text.allDocuments}</Select.Item>
					{#each folders as folder (folder.code)}
						<Select.Item value={folder.code} label={folder.name} class={folder.isChild ? 'pl-6' : ''}>{folder.name}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>

			<div class="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 id="guest-room-folder" class="text-lg font-semibold">{selectedFolder?.name ?? text.allDocuments}</h2>
					<p class="text-muted-foreground text-xs tabular-nums">{visibleDocuments.length} {text.documentCount}</p>
				</div>
				<InputGroup.Root class="bg-background w-full sm:w-72">
					<InputGroup.Addon><SearchIcon /></InputGroup.Addon>
					<InputGroup.Input type="search" placeholder={text.search} aria-label={text.search} bind:value={query} />
				</InputGroup.Root>
			</div>

			<div class="bg-background overflow-hidden rounded-xl border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head class="pl-4">{text.name}</Table.Head>
							<Table.Head class="max-md:hidden">{text.category}</Table.Head>
							<Table.Head class="w-28 max-sm:hidden">{text.date}</Table.Head>
							<Table.Head class="w-16 pr-4 max-sm:hidden">{text.fileType}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each visibleDocuments as document (document.id)}
							{@const visual = fileVisual(sharedFileName(document) ?? '')}
							<Table.Row class="cursor-pointer" onclick={() => (openID = document.id)}>
								<Table.Cell class="max-w-0 py-3 pl-4">
									<div class="flex items-center gap-3">
										<visual.icon class="size-5 shrink-0 {visual.colorClass}" />
										<div class="min-w-0">
											<button type="button" class="block max-w-full truncate text-left font-medium hover:underline"
												>{document.title}</button
											>
											{#if document.summary}<p class="text-muted-foreground truncate text-xs">{document.summary}</p>{/if}
										</div>
									</div>
								</Table.Cell>
								<Table.Cell class="text-muted-foreground max-w-48 truncate max-md:hidden"
									>{categoryLabel(document.category_code)}</Table.Cell
								>
								<Table.Cell class="text-muted-foreground tabular-nums max-sm:hidden">{document.document_date ?? ''}</Table.Cell>
								<Table.Cell class="text-muted-foreground pr-4 text-xs max-sm:hidden">{document.extension?.toUpperCase() ?? ''}</Table.Cell>
							</Table.Row>
						{:else}
							<Table.Row>
								<Table.Cell colspan={4} class="text-muted-foreground py-12 text-center">
									{query ? text.noResults : text.empty}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		</section>
	</div>
</div>

<DocumentViewer
	documents={viewerDocuments}
	bind:openID
	readSource={(document) =>
		previewSourceOf(document.fileName ?? '', (derivedFileName) => signFile(document.id, derivedFileName), canDownload ?? true)}
	onDownload={canDownload === false ? undefined : (document) => onDownload(document.id)}
	{isDownloading}
/>
</Tooltip.Provider>

<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import FileBrowserList, { type FileBrowserEntry } from '$lib/components/file-browser-list.svelte';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { invokeTool } from '$lib/public-api-call';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { categoryName } from '$lib/data-room/model';
	import {
		dataRoomFolderEntries,
		documentCategoryLabel,
		documentFileName,
		type DataRoomDocument
	} from '$lib/data-room/browser';
	import { dataRoomGetResultSchema, companyDocumentListResultSchema } from '$lib/data-room/schemas';
	import { dataRoomText } from '$lib/data-room/text';
	import DocumentViewer from '$lib/data-room/document-viewer.svelte';
	import { previewSourceOf, type ViewerDocument } from '$lib/data-room/viewer';
	import ShareLinks from './share-links.svelte';

	const text = createPageText(dataRoomText);
	let room = $state<z.infer<typeof dataRoomGetResultSchema> | null>(null);
	let documents = $state<DataRoomDocument[]>([]);
	let selectedCode = $state('');
	let openID = $state<string | null>(null);
	let isDownloading = $state(false);
	let isLoading = $state(true);
	let errorMessage = $state('');
	let loadGeneration = 0;
	const categories = $derived(room?.categories ?? []);
	const selectedCategory = $derived(categories.find((category) => category.code === selectedCode));
	const parentCategory = $derived(
		categories.find((category) => category.code === selectedCategory?.parent)
	);
	const entries = $derived(
		dataRoomFolderEntries(documents, categories, selectedCode, currentLocale.value)
	);
	const viewerDocuments = $derived(
		documents.filter((document) => document.categoryCode === selectedCode).map(viewerDocumentOf)
	);
	const breadcrumbs = $derived([
		{ code: '', name: text.title },
		...(parentCategory
			? [{ code: parentCategory.code, name: categoryName(parentCategory, currentLocale.value) }]
			: []),
		...(selectedCategory
			? [{ code: selectedCategory.code, name: categoryName(selectedCategory, currentLocale.value) }]
			: [])
	]);

	async function load() {
		const generation = ++loadGeneration;
		isLoading = true;
		errorMessage = '';
		const results = await Promise.allSettled([
			invokeTool('dataroom_get', {}).then((answer) => {
				if (generation !== loadGeneration) return;
				room = dataRoomGetResultSchema.parse(answer);
			}),
			invokeTool('company_document_list', {}).then((answer) => {
				if (generation !== loadGeneration) return;
				documents = companyDocumentListResultSchema.parse(answer).documents;
			})
		]);
		if (generation !== loadGeneration) return;
		for (const result of results) {
			if (result.status === 'rejected') errorMessage = result.reason instanceof Error ? result.reason.message : text.loadFailed;
		}
		isLoading = false;
	}

	function viewerDocumentOf(document: DataRoomDocument): ViewerDocument {
		const details: [string, string][] = [
			[text.category, documentCategoryLabel(document, categories, currentLocale.value)],
			[text.date, document.date ?? ''],
			[text.documentNumber, document.documentNumber ?? ''],
			[text.counterpart, document.counterpart ?? ''],
			[text.status, document.status ?? ''],
			[text.tags, document.tags.join(' · ')]
		];
		return {
			id: document.documentID,
			title: document.title,
			fileName: document.storagePath ? documentFileName(document) : null,
			category: documentCategoryLabel(document, categories, currentLocale.value),
			date: document.date ?? '',
			summary: document.summary ?? '',
			details: details.filter(([, value]) => value)
		};
	}

	async function signedURL(documentID: string, derivedFileName?: string): Promise<string> {
		const answer = await invokeTool('company_document_download', {
			documentHint: documentID,
			...(derivedFileName ? { fileName: derivedFileName } : {})
		});
		return z.object({ downloadURL: z.string().url() }).parse(answer).downloadURL;
	}

	async function download(document: ViewerDocument) {
		isDownloading = true;
		try {
			const link = window.document.createElement('a');
			link.href = await signedURL(document.id);
			link.download = document.fileName ?? document.title;
			link.rel = 'noopener';
			link.click();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.loadFailed);
		} finally {
			isDownloading = false;
		}
	}

	function openCategory(code: string) {
		selectedCode = code;
		openID = null;
	}
	function openEntry(entry: FileBrowserEntry) {
		if (entry.isDirectory) {
			openCategory(entry.id);
			return;
		}
		openID = entry.id;
	}
	onMount(load);
</script>

<svelte:head><title>{text.title}</title></svelte:head>

<div class="flex h-full min-h-0">
	<div class="min-w-0 flex-1 overflow-auto p-4 md:p-6">
		<div class="mb-4 flex items-center justify-between gap-3">
			<Breadcrumb.Root
				><Breadcrumb.List
					>{#each breadcrumbs as crumb, index (crumb.code)}{#if index > 0}<Breadcrumb.Separator
							/>{/if}<Breadcrumb.Item
							>{#if index === breadcrumbs.length - 1}<Breadcrumb.Page>{crumb.name}</Breadcrumb.Page
								>{:else}<Breadcrumb.Link onclick={() => openCategory(crumb.code)}
									>{crumb.name}</Breadcrumb.Link
								>{/if}</Breadcrumb.Item
						>{/each}</Breadcrumb.List
				></Breadcrumb.Root
			>
			<TooltipIconButton label={text.refresh} variant="ghost" size="icon-sm" onclick={load}
				><RefreshCwIcon /></TooltipIconButton
			>
			{#if room}<ShareLinks />{/if}
		</div>
		{#if errorMessage}<div role="alert" class="mb-4 flex items-center justify-between gap-3">
				<p class="text-destructive text-sm">{errorMessage}</p>
				<Button variant="outline" size="sm" onclick={load}>{text.retry}</Button>
			</div>{/if}
		<div class="flex items-start gap-4">
			<FileBrowserList
				{entries}
				{isLoading}
				hasLoadError={Boolean(errorMessage)}
				title={text.title}
				nameLabel={text.name}
				countBadgeVariant="secondary"
				dateLabel={text.date}
				emptyLabel={text.empty}
				selectedID={openID ?? undefined}
				onSelect={openEntry}
			/>
		</div>
	</div>
</div>

<DocumentViewer
	documents={viewerDocuments}
	bind:openID
	readSource={(document) =>
		previewSourceOf(document.fileName ?? '', (derivedFileName) => signedURL(document.id, derivedFileName), true)}
	onDownload={download}
	{isDownloading}
/>

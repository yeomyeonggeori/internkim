<script lang="ts">
	import { onMount } from 'svelte';
	import type { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import FileBrowserList, { type FileBrowserEntry } from '$lib/components/file-browser-list.svelte';
	import FileBrowserPreview from '$lib/components/file-browser-preview.svelte';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { invokeTool } from '$lib/public-api-call';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { categoryName } from '$lib/data-room/model';
	import {
		dataRoomFolderEntries,
		documentCategoryLabel,
		type DataRoomDocument
	} from '$lib/data-room/browser';
	import { dataRoomGetResultSchema, companyDocumentListResultSchema } from '$lib/data-room/schemas';
	import { dataRoomText } from '$lib/data-room/text';
	import DocumentDetail from './document-detail.svelte';
	import ShareLinks from './share-links.svelte';

	const text = createPageText(dataRoomText);
	let room = $state<z.infer<typeof dataRoomGetResultSchema> | null>(null);
	let documents = $state<DataRoomDocument[]>([]);
	let selectedCode = $state('');
	let selectedDocument = $state<DataRoomDocument | null>(null);
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
				selectedDocument = null;
			})
		]);
		if (generation !== loadGeneration) return;
		for (const result of results) {
			if (result.status === 'rejected') errorMessage = result.reason instanceof Error ? result.reason.message : text.loadFailed;
		}
		isLoading = false;
	}

	function openCategory(code: string) {
		selectedCode = code;
		selectedDocument = null;
	}
	function openEntry(entry: FileBrowserEntry) {
		if (entry.isDirectory) {
			openCategory(entry.id);
			return;
		}
		selectedDocument = documents.find((document) => document.documentID === entry.id) ?? null;
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
			{#if room}<ShareLinks {room} />{/if}
		</div>
		{#if errorMessage}<div role="alert" class="mb-4 flex items-center justify-between gap-3">
				<p class="text-destructive text-sm">{errorMessage}</p>
				<Button variant="outline" size="sm" onclick={load}>{text.retry}</Button>
			</div>{/if}
		<div class="flex items-start gap-4">
			<FileBrowserList
				{entries}
				{isLoading}
				title={text.title}
				nameLabel={text.name}
				secondaryLabel={text.category}
				isSecondaryBadge
				dateLabel={text.date}
				emptyLabel={text.empty}
				selectedID={selectedDocument?.documentID}
				onSelect={openEntry}
			/>
			<FileBrowserPreview
				isOpen={selectedDocument !== null}
				title={selectedDocument?.title ?? text.title}
				onClose={() => (selectedDocument = null)}
			>
				{#if selectedDocument}{#key selectedDocument.documentID}<DocumentDetail
							document={selectedDocument}
							category={documentCategoryLabel(selectedDocument, categories, currentLocale.value)}
							onClose={() => (selectedDocument = null)}
						/>{/key}{/if}
			</FileBrowserPreview>
		</div>
	</div>
</div>

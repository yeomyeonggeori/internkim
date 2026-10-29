<script lang="ts">
	import RefreshButton from '$lib/components/refresh-button.svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import AnimatedHeight from '$lib/components/animated-height.svelte';
	import FileBrowserList, { type FileBrowserEntry } from '$lib/components/file-browser-list.svelte';
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
	import { dataRoomCompanySchema, isEmptyCompany, type DataRoomCompany } from '$lib/data-room/company';
	import { dataRoomCompanyText } from '$lib/data-room/company-text';
	import CurrencySwitcher from '$lib/components/currency-switcher.svelte';
	import CompanyOverview from '$lib/data-room/company-overview.svelte';
	import CompanyOverviewSkeleton from '$lib/data-room/company-overview-skeleton.svelte';
	import FolderBreadcrumb from '$lib/data-room/folder-breadcrumb.svelte';
	import { dataRoomRequest } from '$lib/data-room/guest';
	import DocumentViewer from '$lib/data-room/document-viewer.svelte';
	import { previewSourceOf, type ViewerDocument } from '$lib/data-room/viewer';
	import { formatFileSize, formatModified } from '$lib/files/view';
	import ShareLinks from './share-links.svelte';
	import SharePreview from './share-preview.svelte';

	const text = createPageText(dataRoomText);
	let room = $state<z.infer<typeof dataRoomGetResultSchema> | null>(null);
	let documents = $state<DataRoomDocument[]>([]);
	let company = $state<DataRoomCompany | null>(null);
	let selectedCode = $state('');
	let openID = $state<string | null>(null);
	let isDownloading = $state(false);
	let isLoading = $state(true);
	let errorMessage = $state('');
	let loadGeneration = 0;
	const categories = $derived(room?.categories ?? []);
	const companyText = $derived(dataRoomCompanyText[currentLocale.value]);
	const hasCompany = $derived(company !== null && !isEmptyCompany(company));
	const isCompanyPending = $derived(company === null && isLoading);
	const showsCompany = $derived(hasCompany || isCompanyPending);
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
		{ code: '', name: companyText.documents },
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
			}),
			dataRoomRequest('/api/v1/data-room/company').then((answer) => {
				if (generation !== loadGeneration) return;
				company = dataRoomCompanySchema.parse(answer);
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
			[text.tags, document.tags.join(' · ')],
			[text.size, document.sizeBytes === null ? '' : formatFileSize(document.sizeBytes)],
			[text.created, formatModified(document.createdAt, currentLocale.value)],
			[text.modified, formatModified(document.updatedAt, currentLocale.value)]
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

<div class="min-h-full w-full">
	<div class="grid max-w-5xl min-w-0 gap-12 p-4 md:p-6">
		<div class="grid min-w-0 gap-4">
			<div class="flex items-center justify-between gap-3">
				{#if selectedCode}<FolderBreadcrumb crumbs={breadcrumbs} onOpen={openCategory} />{:else}<h2 class="text-xl font-semibold">{companyText.documents}</h2>{/if}
				<div class="flex items-center gap-2">
					<RefreshButton label={text.refresh} variant="ghost" size="icon-sm" refresh={load} />
					{#if room}<SharePreview /><ShareLinks />{/if}
				</div>
			</div>
			{#if errorMessage}<div role="alert" class="flex items-center justify-between gap-3">
					<p class="text-destructive text-sm">{errorMessage}</p>
					<Button variant="outline" size="sm" onclick={load}>{text.retry}</Button>
				</div>{/if}
			<AnimatedHeight>
				<FileBrowserList
					{entries}
					{isLoading}
					hasLoadError={Boolean(errorMessage)}
					title={text.title}
					nameLabel={text.name}
					countBadgeVariant="secondary"
					columns={[
						{ label: text.size, width: 'narrow' },
						{ label: text.created, width: 'wide' },
						{ label: text.modified, width: 'wide' }
					]}
					emptyLabel={text.empty}
					selectedID={openID ?? undefined}
					onSelect={openEntry}
				/>
			</AnimatedHeight>
		</div>
		{#if showsCompany}
			<section class="grid gap-4" aria-labelledby="data-room-company">
				<div class="flex items-center justify-between gap-3">
					<h2 id="data-room-company" class="text-xl font-semibold">{companyText.company}</h2>
					<CurrencySwitcher label={companyText.currency.label} asRecorded={companyText.currency.asRecorded} />
				</div>
				{#if company && hasCompany}<CompanyOverview {company} />{:else}<CompanyOverviewSkeleton />{/if}
			</section>
		{/if}
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

<script lang="ts">
	import RefreshButton from '$lib/components/refresh-button.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import * as InputGroup from '$lib/components/ui/input-group';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import AnimatedHeight from '$lib/components/animated-height.svelte';
	import FileBrowserList, { type FileBrowserEntry } from '$lib/components/file-browser-list.svelte';
	import CurrencySwitcher from '$lib/components/currency-switcher.svelte';
	import { LanguageSwitcher } from '$lib/components/ui/language-switcher';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { currentLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import CompanyOverview from './company-overview.svelte';
	import { isEmptyCompany, localized } from './company';
	import { dataRoomCompanyText } from './company-text';
	import DocumentViewer from './document-viewer.svelte';
	import FolderBreadcrumb from './folder-breadcrumb.svelte';
	import {
		listedDocuments,
		sharedCategoryName,
		sharedFileName,
		subfolderEntries,
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
		onRefresh,
		fillsViewport = true
	}: {
		room: SharedRoom;
		canDownload?: boolean;
		expiresAt?: string;
		signFile: (documentID: string, derivedFileName?: string) => Promise<string>;
		onDownload: (documentID: string, fileName: string) => void;
		isDownloading?: boolean;
		fillsViewport?: boolean;
		onRefresh: () => void;
	} = $props();

	const text = createPageText(dataRoomText);
	const shellText = createPageText(appShellText);
	let selectedCode = $state('');
	let query = $state('');
	let openID = $state<string | null>(null);
	const hasCompany = $derived(!isEmptyCompany(room.company));
	const companyText = $derived(dataRoomCompanyText[currentLocale.value]);
	const roomTitle = $derived(localized(room.company.profile.name, currentLocale.value) || text.sharedRoom);
	const selectedCategory = $derived(room.categories.find((category) => category.code === selectedCode));
	const parentCategory = $derived(room.categories.find((category) => category.code === selectedCategory?.parent));
	const visibleDocuments = $derived(listedDocuments(room, selectedCode, query));
	const entries = $derived([
		...(query ? [] : subfolderEntries(room, selectedCode, currentLocale.value)),
		...visibleDocuments.map(documentEntryOf)
	]);
	const viewerDocuments = $derived(visibleDocuments.map(viewerDocumentOf));
	const breadcrumbs = $derived([
		{ code: '', name: companyText.documents },
		...[parentCategory, selectedCategory]
			.filter((category) => category !== undefined)
			.map((category) => ({ code: category.code, name: sharedCategoryName(category, currentLocale.value) }))
	]);
	const openedAt = new Date();
	const watermark = $derived(
		`${room.viewer ?? text.publicViewer} · ${openedAt.toLocaleString(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US', {
			dateStyle: 'medium',
			timeStyle: 'short'
		})}`
	);
	const expiryLabel = $derived(
		expiresAt
			? new Date(expiresAt).toLocaleString(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US', {
					month: 'short',
					day: 'numeric',
					hour: 'numeric',
					minute: '2-digit'
				})
			: ''
	);
	const accessLine = $derived(
		[
			canDownload === undefined ? '' : canDownload ? text.downloadAllowed : text.viewOnly,
			expiryLabel ? `${text.expires} ${expiryLabel}` : ''
		]
			.filter(Boolean)
			.join(' · ')
	);

	function categoryLabel(code: string): string {
		const category = room.categories.find((candidate) => candidate.code === code);
		return category ? sharedCategoryName(category, currentLocale.value) : code;
	}

	function documentEntryOf(document: SharedDocument): FileBrowserEntry {
		return {
			id: document.id,
			name: document.title,
			fileName: sharedFileName(document) ?? undefined,
			cells: [document.extension?.toUpperCase() ?? '', document.document_date ?? '']
		};
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

	function openEntry(entry: FileBrowserEntry) {
		if (entry.isDirectory) {
			openFolder(entry.id);
			return;
		}
		openID = entry.id;
	}
</script>

<Tooltip.Provider delayDuration={120}>
<div class={cn('bg-muted/40 flex flex-1 flex-col', fillsViewport && 'min-h-dvh')}>
	<header class="bg-background sticky top-0 z-10 border-b">
		<div class="mx-auto flex w-full max-w-5xl items-center gap-3 px-4 py-3">
			<img src={room.company.logo ?? '/favicon.svg'} alt="" class="size-8 shrink-0 object-contain" />
			<div class="min-w-0 flex-1">
				<h1 class="truncate text-base leading-tight font-semibold">{roomTitle}</h1>
				{#if accessLine}<p class="text-muted-foreground mt-0.5 truncate text-xs">{accessLine}</p>{/if}
			</div>
			<RefreshButton label={text.refresh} variant="ghost" size="icon-sm" refresh={onRefresh} />
			<LanguageSwitcher
				variant="ghost"
				languages={localeOptions.map((option) => ({ code: option.value, label: option.label }))}
				value={currentLocale.value}
				ariaLabel={shellText.changeLanguage}
				onChange={(code) => (code === 'ko' || code === 'en') && setLocale(code)}
			/>
		</div>
	</header>

	<div class="mx-auto grid w-full max-w-5xl flex-1 content-start gap-12 px-4 py-6">
		<div class="grid min-w-0 gap-4">
			<div class="flex flex-wrap items-center justify-between gap-3">
				{#if selectedCode}<FolderBreadcrumb crumbs={breadcrumbs} onOpen={openFolder} />{:else}<h2 class="text-xl font-semibold">{companyText.documents}</h2>{/if}
				<InputGroup.Root class="bg-background w-full sm:w-72">
					<InputGroup.Addon><SearchIcon /></InputGroup.Addon>
					<InputGroup.Input type="search" placeholder={text.search} aria-label={text.search} bind:value={query} />
				</InputGroup.Root>
			</div>
			<AnimatedHeight>
				<FileBrowserList
					{entries}
					title={companyText.documents}
					nameLabel={text.name}
					countBadgeVariant="secondary"
					columns={[
						{ label: text.fileType, width: 'narrow' },
						{ label: text.date, width: 'wide' }
					]}
					emptyLabel={query ? text.noResults : text.empty}
					selectedID={openID ?? undefined}
					onSelect={openEntry}
				/>
			</AnimatedHeight>
			<p class="text-muted-foreground text-xs">{text.confidential}</p>
		</div>
		{#if hasCompany}
			<section class="grid gap-4" aria-labelledby="guest-room-company">
				<div class="flex items-center justify-between gap-3">
					<h2 id="guest-room-company" class="text-xl font-semibold">{companyText.company}</h2>
					<CurrencySwitcher label={companyText.currency.label} asRecorded={companyText.currency.asRecorded} />
				</div>
				<CompanyOverview company={room.company} sharedWith={room.viewer} />
			</section>
		{/if}
	</div>
</div>

<DocumentViewer
	documents={viewerDocuments}
	bind:openID
	readSource={(document) =>
		previewSourceOf(document.fileName ?? '', (derivedFileName) => signFile(document.id, derivedFileName), canDownload ?? true)}
	onDownload={canDownload === false ? undefined : (document) => onDownload(document.id, document.fileName ?? document.title)}
	{isDownloading}
	{watermark}
/>
</Tooltip.Provider>

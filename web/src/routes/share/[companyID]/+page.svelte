<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Select from '$lib/components/ui/select';
	import { dataRoomRequest, dataRoomFile, sharedDataRoomSchema } from '$lib/data-room/guest';

	let room = $state<z.infer<typeof sharedDataRoomSchema> | null>(null);
	let selectedCode = $state('');
	let selectedDocument = $state('');
	let text = $state('');
	let failure = $state('');
	const visibleDocuments = $derived(room?.documents.filter((document) => !selectedCode
		|| document.category_code === selectedCode
		|| room?.categories.find((category) => category.code === document.category_code)?.parent === selectedCode) ?? []);

	async function openDocument(documentID: string, isDownload = false) {
		failure = '';
		try {
			const companyID = z.string().uuid().parse(page.params.companyID);
			const fileURL = await dataRoomFile(companyID, documentID, isDownload ? undefined : 'content.txt');
			if (isDownload) { window.location.assign(fileURL); return; }
			const response = await fetch(fileURL);
			if (!response.ok) throw new Error('The text preview is not available.');
			text = await response.text();
			selectedDocument = documentID;
		} catch (error) { failure = error instanceof Error ? error.message : String(error); }
	}

	onMount(async () => {
		try { room = sharedDataRoomSchema.parse(await dataRoomRequest(`/api/v1/data-room/${page.params.companyID}`)); }
		catch (error) { failure = error instanceof Error ? error.message : String(error); }
	});
</script>

<svelte:head><title>Shared data room</title></svelte:head>
<main class="mx-auto max-w-6xl p-4 md:p-6 md:py-10">
	<h1 class="mb-6 text-2xl font-semibold">Shared data room</h1>
	{#if failure}<p role="alert" class="mb-4 text-sm text-destructive">{failure}</p>{/if}
	<div class="grid gap-6 md:grid-cols-[220px_1fr]">
		<div class="grid gap-1.5 md:hidden">
			<label for="shared-category" class="text-sm font-medium">Categories</label>
			<Select.Root type="single" value={selectedCode || 'all'} onValueChange={(value) => selectedCode = value === 'all' ? '' : value}>
				<Select.Trigger id="shared-category" class="w-full min-w-0"><span class="min-w-0 truncate">{room?.categories.find((category) => category.code === selectedCode)?.name ?? 'All documents'}</span></Select.Trigger>
				<Select.Content><Select.Group>
					<Select.Item value="all">All documents</Select.Item>
					{#each room?.categories ?? [] as category (category.code)}<Select.Item value={category.code} label={category.name}>{category.code} · {category.name}</Select.Item>{/each}
				</Select.Group></Select.Content>
			</Select.Root>
		</div>
		<nav aria-label="Categories" class="hidden space-y-1 md:block">
			<Button class="w-full justify-start" variant={selectedCode ? 'ghost' : 'secondary'} onclick={() => selectedCode = ''}>All documents</Button>
			{#each room?.categories.filter((category) => !category.parent) ?? [] as parent (parent.code)}
				<Button class="w-full justify-start" variant={selectedCode === parent.code ? 'secondary' : 'ghost'} onclick={() => selectedCode = parent.code}>{parent.code} · {parent.name}</Button>
				{#each room?.categories.filter((category) => category.parent === parent.code) ?? [] as child (child.code)}
					<Button class="w-full justify-start pl-5" size="sm" variant={selectedCode === child.code ? 'secondary' : 'ghost'} onclick={() => selectedCode = child.code}>{child.code} · {child.name}</Button>
				{/each}
			{/each}
		</nav>
			<section aria-label="Documents" class="min-w-0 space-y-4">
				{#if !room && !failure}<div role="status" aria-label="Loading documents" aria-busy="true" class="grid gap-4">{#each [0, 1, 2] as row (row)}<div aria-hidden="true" class="grid gap-3 rounded-lg border p-4"><div class="flex justify-between gap-4"><Skeleton class="h-4 w-2/3" /><Skeleton class="h-3 w-12" /></div><Skeleton class="h-4 w-full" /><Skeleton class="h-4 w-4/5" /><div class="flex gap-2"><Skeleton class="h-8 w-16" /><Skeleton class="h-8 w-24" /></div></div>{/each}</div>{/if}
				{#each visibleDocuments as document (document.id)}
				<article class="rounded-lg border p-4">
					<div class="flex items-start justify-between gap-4"><h2 class="min-w-0 break-words text-sm font-semibold">{document.title}</h2><span class="shrink-0 font-mono text-xs text-muted-foreground">{document.category_code}</span></div>
					<p class="mt-2 text-sm text-muted-foreground">{document.summary ?? ''}</p>
					<div class="mt-3 flex gap-2"><Button variant="outline" size="sm" onclick={() => openDocument(document.id)}>Read</Button><Button variant="ghost" size="sm" onclick={() => openDocument(document.id, true)}>Download</Button></div>
					{#if selectedDocument === document.id}<pre class="mt-4 max-h-[70vh] overflow-auto whitespace-pre-wrap break-words border-t pt-4 text-sm font-sans">{text}</pre>{/if}
				</article>
				{:else}{#if room}<p class="rounded-lg border p-8 text-sm text-muted-foreground">No documents are available to you.</p>{/if}{/each}
		</section>
	</div>
</main>

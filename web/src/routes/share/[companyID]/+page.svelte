<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
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
<main class="mx-auto max-w-6xl p-6 md:py-10">
	<h1 class="mb-6 text-2xl font-semibold">Shared data room</h1>
	{#if failure}<p role="alert" class="mb-4 text-sm text-destructive">{failure}</p>{/if}
	<div class="grid gap-6 md:grid-cols-[220px_1fr]">
		<nav aria-label="Categories" class="space-y-1">
			<Button class="w-full justify-start" variant={selectedCode ? 'ghost' : 'secondary'} onclick={() => selectedCode = ''}>All documents</Button>
			{#each room?.categories.filter((category) => !category.parent) ?? [] as parent (parent.code)}
				<Button class="w-full justify-start" variant={selectedCode === parent.code ? 'secondary' : 'ghost'} onclick={() => selectedCode = parent.code}>{parent.code} · {parent.name}</Button>
				{#each room?.categories.filter((category) => category.parent === parent.code) ?? [] as child (child.code)}
					<Button class="w-full justify-start pl-5" size="sm" variant={selectedCode === child.code ? 'secondary' : 'ghost'} onclick={() => selectedCode = child.code}>{child.code} · {child.name}</Button>
				{/each}
			{/each}
		</nav>
		<section aria-label="Documents" class="min-w-0 space-y-4">
			{#each visibleDocuments as document (document.id)}
				<article class="rounded-lg border p-4">
					<div class="flex items-start justify-between gap-4"><h2 class="text-sm font-semibold">{document.title}</h2><span class="font-mono text-xs text-muted-foreground">{document.category_code}</span></div>
					<p class="mt-2 text-sm text-muted-foreground">{document.summary ?? ''}</p>
					<div class="mt-3 flex gap-2"><Button variant="outline" size="sm" onclick={() => openDocument(document.id)}>Read</Button><Button variant="ghost" size="sm" onclick={() => openDocument(document.id, true)}>Download</Button></div>
					{#if selectedDocument === document.id}<pre class="mt-4 max-h-[70vh] overflow-auto whitespace-pre-wrap border-t pt-4 text-sm font-sans">{text}</pre>{/if}
				</article>
			{:else}<p class="rounded-lg border p-8 text-sm text-muted-foreground">{room ? 'No documents are available to you.' : 'Loading…'}</p>{/each}
		</section>
	</div>
</main>

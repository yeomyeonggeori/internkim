<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import RefreshButton from '$lib/components/refresh-button.svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { z } from 'zod';
	import GuestRoom from '$lib/data-room/guest-room.svelte';
	import type { SharedRoom } from '$lib/data-room/guest-room';
	import { dataRoomRequest, dataRoomDownload, dataRoomFile, sharedDataRoomSchema } from '$lib/data-room/guest';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { dataRoomText } from '$lib/data-room/text';

	const text = createPageText(dataRoomText);
	let room = $state<SharedRoom | null>(null);
	let failure = $state('');
	const companyID = $derived(z.string().uuid().parse(page.params.companyID));

	async function load() {
		failure = '';
		try {
			room = sharedDataRoomSchema.parse(await dataRoomRequest(`/api/v1/data-room/${companyID}`));
		} catch (error) {
			failure = error instanceof Error ? error.message : text.loadFailed;
		}
	}

	async function download(documentID: string, fileName: string) {
		failure = '';
		try {
			await dataRoomDownload(companyID, documentID, fileName);
		} catch (error) {
			failure = error instanceof Error ? error.message : text.loadFailed;
		}
	}

	onMount(load);
</script>

<svelte:head
	><title>{text.sharedRoom}</title><meta name="robots" content="noindex,nofollow" /></svelte:head
>

{#if room}
	<GuestRoom
		{room}
		signFile={(documentID, derivedFileName) => dataRoomFile(companyID, documentID, derivedFileName)}
		onDownload={download}
		onRefresh={load}
	/>
{:else if !failure}
	<div role="status" aria-label={text.title} aria-busy="true" class="flex min-h-dvh flex-col">
		<header aria-hidden="true" class="bg-background sticky top-0 z-10 border-b">
			<div class="mx-auto flex w-full max-w-5xl items-center gap-3 px-4 py-3">
				<Skeleton class="size-8 shrink-0" />
				<div class="grid min-w-0 flex-1 gap-1"><Skeleton class="h-4 w-40" /><Skeleton class="h-3 w-56 max-w-full" /></div>
				<Tooltip.Provider><RefreshButton label={text.refresh} variant="ghost" size="icon-sm" refresh={load} disabled /></Tooltip.Provider>
			</div>
		</header>
		<div aria-hidden="true" class="mx-auto grid w-full max-w-5xl gap-4 px-4 py-6">
			<Skeleton class="h-7 w-24" />
			<Skeleton class="h-48 w-full" />
			<Skeleton class="mt-8 h-7 w-28" />
			<Skeleton class="h-24 w-full" />
			<div class="grid gap-4 sm:grid-cols-3">{#each [0, 1, 2] as tile (tile)}<Skeleton class="h-28 w-full" />{/each}</div>
		</div>
	</div>
{/if}
{#if failure}<p
		role="alert"
		class="bg-background text-destructive fixed inset-x-4 bottom-4 mx-auto max-w-md rounded-lg border p-3 text-sm shadow-lg"
	>
		{failure}
	</p>{/if}

<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { z } from 'zod';
	import { Spinner } from '$lib/components/ui/spinner';
	import GuestRoom from '$lib/data-room/guest-room.svelte';
	import type { SharedRoom } from '$lib/data-room/guest-room';
	import { dataRoomRequest, dataRoomFile, sharedDataRoomSchema } from '$lib/data-room/guest';
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

	async function download(documentID: string) {
		failure = '';
		try {
			window.location.assign(await dataRoomFile(companyID, documentID));
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
	<div role="status" aria-label={text.title} class="flex min-h-dvh items-center justify-center"><Spinner /></div>
{/if}
{#if failure}<p
		role="alert"
		class="bg-background text-destructive fixed inset-x-4 bottom-4 mx-auto max-w-md rounded-lg border p-3 text-sm shadow-lg"
	>
		{failure}
	</p>{/if}

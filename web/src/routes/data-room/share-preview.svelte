<script lang="ts">
	import { z } from 'zod';
	import XIcon from '@lucide/svelte/icons/x';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Spinner } from '$lib/components/ui/spinner';
	import GuestRoom from '$lib/data-room/guest-room.svelte';
	import { savedDownload } from '$lib/data-room/guest';
	import { dataRoomLinksSchema } from '$lib/data-room/links';
	import { circleListResultSchema, previewedDataRoomSchema } from '$lib/data-room/schemas';
	import { invokeTool, memberAccessToken } from '$lib/public-api-call';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { dataRoomSharingText } from '$lib/data-room/sharing-text';
	import { dataRoomText } from '$lib/data-room/text';
	import { shareableCircleItems, type CircleItem } from './circle-items';

	type PreviewedRoom = z.infer<typeof previewedDataRoomSchema>;
	const text = createPageText(dataRoomSharingText);
	const browserText = createPageText(dataRoomText);
	const fieldID = $props.id();
	let isOpen = $state(false);
	let isBusy = $state(false);
	let errorMessage = $state('');
	let circleItems = $state<CircleItem[]>([]);
	let downloadableCircleIDs = $state<string[]>([]);
	let circleID = $state('investor');
	let canDownload = $state(true);
	let room = $state<PreviewedRoom | null>(null);
	const circleLabel = $derived(circleItems.find((item) => item.value === circleID)?.label ?? '');
	const endpoint = $derived(`/api/v1/data-room/preview/${encodeURIComponent(circleID)}`);

	async function previewFetch(parameters: Record<string, string> = {}): Promise<Response> {
		const query = new URLSearchParams({ ...parameters, ...(canDownload ? { canDownload: '1' } : {}) });
		const response = await fetch(`${endpoint}?${query}`, {
			cache: 'no-store',
			headers: { Authorization: `Bearer ${await memberAccessToken()}` }
		});
		if (response.ok) return response;
		const refusal = z.object({ message: z.string() }).safeParse(await response.json().catch(() => null));
		throw new Error(refusal.success ? refusal.data.message : text.failure);
	}

	async function load() {
		isBusy = true;
		errorMessage = '';
		try {
			const answer = previewedDataRoomSchema.parse(await (await previewFetch()).json());
			room = { ...answer, viewer: circleLabel ? `${text.previewViewer} · ${circleLabel}` : text.previewViewer };
		} catch (error) {
			room = null;
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function open() {
		isOpen = true;
		isBusy = true;
		errorMessage = '';
		try {
			const [links, circleList] = await Promise.all([
				invokeTool('dataroom_links_get', {}).then((value) => dataRoomLinksSchema.parse(value)),
				invokeTool('circle_list', {}).then((value) => circleListResultSchema.parse(value))
			]);
			circleItems = shareableCircleItems(circleList.circles, links.shareableCircleIDs, currentLocale.value);
			downloadableCircleIDs = links.downloadableCircleIDs;
			if (!circleItems.some((item) => item.value === circleID)) circleID = circleItems[0]?.value ?? '';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
		if (circleID) await load();
	}

	function close() {
		isOpen = false;
		room = null;
	}

	async function signFile(documentID: string, derivedFileName?: string): Promise<string> {
		const response = await previewFetch({ documentID, ...(derivedFileName ? { fileName: derivedFileName } : {}) });
		return z.object({ downloadURL: z.string().url() }).parse(await response.json()).downloadURL;
	}

	async function download(documentID: string, fileName: string) {
		try {
			await savedDownload(await previewFetch({ documentID, download: '1' }), fileName);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		}
	}
</script>

<Button size="sm" variant="outline" onclick={open}>{text.preview}</Button>
{#if isOpen}
	<div role="dialog" aria-modal="true" aria-label={text.previewing} class="bg-background fixed inset-0 z-(--layer-panel) flex flex-col overflow-y-auto overscroll-contain">
		<div class="bg-primary text-primary-foreground flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2 text-sm">
			<p class="font-medium">{text.previewing}</p>
			{#if circleItems.length}<Select.Root
					type="single"
					items={circleItems}
					bind:value={circleID}
					onValueChange={load}
					disabled={isBusy}
					><Select.Trigger size="sm" aria-label={text.circle} class="bg-background text-foreground h-8 min-w-36"
						>{circleLabel}</Select.Trigger
					><Select.Content
						><Select.Group
							>{#each circleItems as item (item.value)}<Select.Item value={item.value} label={item.label}
									>{item.label}</Select.Item
								>{/each}</Select.Group
						></Select.Content
					></Select.Root
				>{/if}
			<div class="flex items-center gap-2">
				<Checkbox
					id="{fieldID}-download"
					class="border-primary-foreground"
					checked={canDownload && downloadableCircleIDs.includes(circleID)}
					disabled={isBusy || !downloadableCircleIDs.includes(circleID)}
					onCheckedChange={(checked) => {
						canDownload = checked === true;
						void load();
					}}
				/><label for="{fieldID}-download">{text.download}</label>
			</div>
			{#if isBusy}<Spinner />{/if}
			<Button
				variant="ghost"
				size="icon-sm"
				class="hover:bg-primary-foreground/10 hover:text-primary-foreground ms-auto"
				aria-label={browserText.close}
				onclick={close}><XIcon /></Button
			>
		</div>
		{#if errorMessage}<p role="alert" class="text-destructive p-4 text-sm">{errorMessage}</p>{/if}
		{#if room}<GuestRoom {room} canDownload={room.canDownload} {signFile} onDownload={download} onRefresh={load} fillsViewport={false} />
		{:else if !isBusy && !circleItems.length && !errorMessage}<p class="text-muted-foreground p-4 text-sm">{text.noCircles}</p>{/if}
	</div>
{/if}

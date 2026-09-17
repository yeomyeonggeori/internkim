<script lang="ts">
	import HashIcon from '@lucide/svelte/icons/hash';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Empty from '$lib/components/ui/empty';
	import { Spinner } from '$lib/components/ui/spinner';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { fetchOpenChannels, joinChannel, type OpenChannel } from '$lib/messenger/messenger-api';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		onJoined
	}: {
		open?: boolean;
		onJoined: (channelID: string) => void;
	} = $props();

	const text = createPageText(channelText);

	let channels = $state<OpenChannel[]>([]);
	let isLoading = $state(false);
	let joiningID = $state<string | null>(null);

	$effect(() => {
		if (!open) return;
		isLoading = true;
		channels = [];
		fetchOpenChannels()
			.then((found) => (channels = found))
			.catch((failure: unknown) =>
				toast.error(failure instanceof Error ? failure.message : text.loadOpenChannelsFailed)
			)
			.finally(() => (isLoading = false));
	});

	async function join(channel: OpenChannel) {
		joiningID = channel.id;
		try {
			await joinChannel(channel.id);
			open = false;
			onJoined(channel.id);
		} catch (failure) {
			toast.error(failure instanceof Error ? failure.message : text.joinChannelFailed);
		} finally {
			joiningID = null;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{text.browseChannels}</Dialog.Title>
		</Dialog.Header>
		{#if isLoading}
			<div class="grid place-items-center py-8"><Spinner class="size-5" /></div>
		{:else if channels.length === 0}
			<Empty.Root>
				<Empty.Header>
					<Empty.Title>{text.noOpenChannels}</Empty.Title>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<ul class="-mx-2 max-h-80 overflow-y-auto">
				{#each channels as channel (channel.id)}
					<li class="flex items-center gap-3 rounded-md px-2 py-2">
						<HashIcon class="text-muted-foreground size-4 shrink-0" />
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm font-medium">{channel.name}</p>
							{#if channel.description}
								<p class="text-muted-foreground truncate text-xs">{channel.description}</p>
							{/if}
						</div>
						<Button size="sm" variant="outline" disabled={joiningID !== null} onclick={() => join(channel)}>
							{#if joiningID === channel.id}<Spinner />{/if}
							{text.joinChannel}
						</Button>
					</li>
				{/each}
			</ul>
		{/if}
	</Dialog.Content>
</Dialog.Root>

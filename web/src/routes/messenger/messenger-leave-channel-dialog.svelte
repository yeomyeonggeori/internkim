<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { ChannelSummary } from '$lib/components/channel/channel-api';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { MessengerRefusal, leaveChannel } from '$lib/messenger/messenger-api';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		channel,
		onLeft
	}: {
		open?: boolean;
		channel: ChannelSummary | null;
		onLeft: (channelID: string) => void;
	} = $props();

	const text = createPageText(channelText);

	let isLeaving = $state(false);

	async function leave() {
		if (!channel) return;
		isLeaving = true;
		try {
			await leaveChannel(channel.id);
			open = false;
			onLeft(channel.id);
		} catch (failure) {
			open = false;
			if (failure instanceof MessengerRefusal && failure.reason === 'last-owner') {
				toast.error(text.lastOwnerCannotLeave);
			} else {
				toast.error(failure instanceof Error ? failure.message : text.leaveChannelFailed);
			}
		} finally {
			isLeaving = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.leaveChannelTitle.replace('{name}', channel?.name ?? '')}</AlertDialog.Title>
			<AlertDialog.Description>{text.leaveChannelDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={isLeaving}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action disabled={isLeaving} onclick={leave}>{text.leaveChannel}</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

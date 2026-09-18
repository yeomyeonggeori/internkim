<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Command from '$lib/components/ui/command';
	import * as Dialog from '$lib/components/ui/dialog';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelMember } from '$lib/components/channel/channel-api';
	import { MessengerRefusal, handOverChannel } from '$lib/messenger/messenger-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		channelID,
		members,
		onHandedOver
	}: {
		open?: boolean;
		channelID: string;
		members: ChannelMember[];
		onHandedOver: () => void;
	} = $props();

	const text = createPageText(channelText);
	const candidates = $derived(members.filter((member) => member.role !== 'owner'));

	let isHandingOver = $state(false);

	async function handOverTo(member: ChannelMember) {
		isHandingOver = true;
		try {
			await handOverChannel(channelID, member.externalID ?? '');
			open = false;
			onHandedOver();
		} catch (failure) {
			onHandedOver();
			toast.error(refusalText(failure));
		} finally {
			isHandingOver = false;
		}
	}

	function refusalText(failure: unknown): string {
		if (failure instanceof MessengerRefusal && failure.reason === 'not-owner') return text.notChannelOwner;
		if (failure instanceof MessengerRefusal && failure.reason === 'still-an-owner') {
			return text.ownershipPartlyHandedOver;
		}
		return failure instanceof Error ? failure.message : text.handOverChannelFailed;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-sm" closeLabel={text.closeChannelMembers}>
		<Dialog.Header>
			<Dialog.Title>{text.handOverChannelTitle}</Dialog.Title>
		</Dialog.Header>
		<Command.Root>
			<Command.Input placeholder={text.searchMembers} />
			<Command.List>
				<Command.Empty>{text.noMatchingMembers}</Command.Empty>
				<Command.Group>
					{#each candidates as member (member.memberID ?? member.externalID)}
						<Command.Item
							value={member.memberID ?? member.externalID ?? ''}
							keywords={[member.name, displayPersonName(member.name)]}
							disabled={isHandingOver}
							onSelect={() => handOverTo(member)}
						>
							<PersonAvatar
								name={member.name}
								seed={member.memberID ?? member.externalID ?? ''}
								memberID={member.memberID ?? ''}
								externalID={member.externalID ?? ''}
								class="size-8"
							/>
							<span class="truncate">{displayPersonName(member.name) || text.unnamedMember}</span>
						</Command.Item>
					{/each}
				</Command.Group>
			</Command.List>
		</Command.Root>
	</Dialog.Content>
</Dialog.Root>

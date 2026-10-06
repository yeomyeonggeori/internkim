<script lang="ts">
	import * as Empty from '$lib/components/ui/empty';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Command from '$lib/components/ui/command';
	import * as Dialog from '$lib/components/ui/dialog';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelMember } from '$lib/components/channel/channel-api';
	import { MessengerRefusal, addChannelOwner, handOverChannel } from '$lib/messenger/messenger-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		channelID,
		members,
		action,
		onHandedOver
	}: {
		open?: boolean;
		channelID: string;
		members: ChannelMember[];
		action: 'add' | 'hand-over';
		onHandedOver: () => void;
	} = $props();

	const text = createPageText(channelText);
	const candidates = $derived(members.filter((member) => member.role !== 'owner'));
	const title = $derived(action === 'hand-over' ? text.handOverChannelTitle : text.addChannelOwnerTitle);

	let isSubmitting = $state(false);

	async function chooseOwner(member: ChannelMember) {
		isSubmitting = true;
		try {
			if (action === 'hand-over') {
				await handOverChannel(channelID, member.externalID ?? '');
			} else {
				await addChannelOwner(channelID, member.externalID ?? '');
			}
			open = false;
			onHandedOver();
		} catch (failure) {
			onHandedOver();
			toast.error(refusalText(failure));
		} finally {
			isSubmitting = false;
		}
	}

	function refusalText(failure: unknown): string {
		if (failure instanceof MessengerRefusal && failure.reason === 'not-owner') return text.notChannelOwner;
		if (action === 'hand-over' && failure instanceof MessengerRefusal && failure.reason === 'still-an-owner') {
			return text.ownershipPartlyHandedOver;
		}
		if (failure instanceof Error) return failure.message;
		return action === 'hand-over' ? text.handOverChannelFailed : text.addChannelOwnerFailed;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-sm" closeLabel={text.closeChannelMembers}>
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
		</Dialog.Header>
		<Command.Root>
			<Command.Input placeholder={text.searchMembers} />
			<Command.List>
				<Command.Empty class="p-0"><Empty.Root class="p-3"><Empty.Header><Empty.Title>{text.noMatchingMembers}</Empty.Title></Empty.Header></Empty.Root></Command.Empty>
				<Command.Group>
					{#each candidates as member (member.memberID ?? member.externalID)}
						<Command.Item
							value={member.memberID ?? member.externalID ?? ''}
							keywords={[member.name, displayPersonName(member.name)]}
							disabled={isSubmitting}
							onSelect={() => chooseOwner(member)}
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

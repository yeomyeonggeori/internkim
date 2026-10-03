<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonMultiSelect from '$lib/components/person-multi-select.svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Spinner } from '$lib/components/ui/spinner';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelMember, ChannelRole } from '$lib/components/channel/channel-api';
	import { fetchChannelCandidates, type ChannelCandidate } from '$lib/messenger/channel-candidates';
	import { MessengerRefusal, addChannelMembers, removeChannelMember } from '$lib/messenger/messenger-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { avatarPresenceOf } from '$lib/messenger/member-presence.svelte';
	import UserMinusIcon from '@lucide/svelte/icons/user-minus';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		channelID,
		members,
		viewerRole,
		onMembersChanged
	}: {
		open?: boolean;
		channelID: string;
		members: ChannelMember[];
		viewerRole?: ChannelRole;
		onMembersChanged: () => void;
	} = $props();

	const text = createPageText(channelText);

	let candidates = $state<ChannelCandidate[]>([]);
	let chosenIDs = $state<string[]>([]);
	let isAdding = $state(false);
	let removing = $state<ChannelMember | null>(null);
	let isRemoving = $state(false);

	const memberExternalIDs = $derived(new Set(members.map((member) => member.externalID)));
	const addable = $derived(candidates.filter((candidate) => !memberExternalIDs.has(candidate.externalID)));

	$effect(() => {
		if (!open) return;
		chosenIDs = [];
		fetchChannelCandidates(text.title)
			.then((found) => (candidates = found))
			.catch(() => (candidates = []));
	});

	function switchChosen(memberID: string) {
		chosenIDs = chosenIDs.includes(memberID)
			? chosenIDs.filter((chosen) => chosen !== memberID)
			: [...chosenIDs, memberID];
	}

	async function addChosen() {
		const chosen = addable.filter((candidate) => chosenIDs.includes(candidate.memberID));
		isAdding = true;
		try {
			const uninvited = await addChannelMembers(
				channelID,
				chosen.map((candidate) => candidate.externalID)
			);
			chosenIDs = [];
			onMembersChanged();
			const names = chosen
				.filter((candidate) => uninvited.includes(candidate.externalID))
				.map((candidate) => displayPersonName(candidate.name));
			if (names.length > 0) toast.warning(text.membersNotAdded.replace('{names}', names.join(', ')));
		} catch (failure) {
			toast.error(failure instanceof Error ? failure.message : text.addMembersFailed);
		} finally {
			isAdding = false;
		}
	}

	function askAboutRemoval(member: ChannelMember) {
		removing = member;
	}

	async function removeThem() {
		if (!removing) return;
		const member = removing;
		isRemoving = true;
		try {
			await removeChannelMember(channelID, member.externalID ?? '');
			removing = null;
			onMembersChanged();
		} catch (failure) {
			removing = null;
			toast.error(removalRefusalText(failure));
		} finally {
			isRemoving = false;
		}
	}

	function removalRefusalText(failure: unknown): string {
		if (failure instanceof MessengerRefusal && failure.reason === 'not-owner') return text.notChannelOwner;
		if (failure instanceof MessengerRefusal && failure.reason === 'target-is-owner') {
			return text.channelOwnerCannotBeRemoved;
		}
		return failure instanceof Error ? failure.message : text.removeChannelMemberFailed;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md" closeLabel={text.closeChannelMembers}>
		<Dialog.Header>
			<Dialog.Title>{text.channelMembersTitle}</Dialog.Title>
		</Dialog.Header>
		<div class="flex items-start gap-2">
			<div class="min-w-0 flex-1">
				<PersonMultiSelect
					selectedIDs={chosenIDs}
					people={addable}
					label={text.addChannelMembers}
					placeholder={text.addMembersPlaceholder}
					onToggle={switchChosen}
					onRemove={switchChosen}
					disabled={isAdding}
				/>
			</div>
			<Button onclick={addChosen} disabled={isAdding || chosenIDs.length === 0}>
				{#if isAdding}<Spinner />{/if}
				{text.addChannelMembers}
			</Button>
		</div>
		<Command.Root>
			<Command.Input placeholder={text.searchMembers} />
			<Command.List>
				<Command.Empty>{text.noMatchingMembers}</Command.Empty>
				<Command.Group heading={text.channelMemberCount.replace('{count}', String(members.length))}>
					{#each members as member (member.memberID ?? member.externalID)}
						<Command.Item
							value={member.memberID ?? member.externalID ?? ''}
							keywords={[member.name, displayPersonName(member.name)]}
						>
							<PersonAvatar
								name={member.name}
								seed={member.memberID ?? member.externalID ?? ''}
								memberID={member.memberID ?? ''}
								externalID={member.externalID ?? ''}
								{...avatarPresenceOf(member.memberID, text)}
								class="size-8"
							/>
							<span class="truncate">{displayPersonName(member.name) || text.unnamedMember}</span>
							{#if viewerRole === 'owner' && member.role === 'member'}
								<Button
									type="button"
									variant="ghost"
									size="icon-sm"
									class="ml-auto shrink-0"
									aria-label={text.removeChannelMember}
									onclick={(event) => {
										event.stopPropagation();
										askAboutRemoval(member);
									}}
								>
									<UserMinusIcon />
								</Button>
							{/if}
						</Command.Item>
					{/each}
				</Command.Group>
			</Command.List>
		</Command.Root>
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root open={removing !== null} onOpenChange={(next) => !next && (removing = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{text.removeChannelMemberTitle.replace(
					'{name}',
					removing ? displayPersonName(removing.name) || text.unnamedMember : ''
				)}
			</AlertDialog.Title>
			<AlertDialog.Description>{text.removeChannelMemberDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={isRemoving}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" disabled={isRemoving} onclick={removeThem}>
				{text.removeChannelMember}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

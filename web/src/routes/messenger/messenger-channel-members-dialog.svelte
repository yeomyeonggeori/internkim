<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonMultiSelect from '$lib/components/person-multi-select.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Spinner } from '$lib/components/ui/spinner';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelMember } from '$lib/components/channel/channel-api';
	import { fetchChannelCandidates, type ChannelCandidate } from '$lib/messenger/channel-candidates';
	import { addChannelMembers } from '$lib/messenger/messenger-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		channelID,
		members,
		onMembersChanged
	}: {
		open?: boolean;
		channelID: string;
		members: ChannelMember[];
		onMembersChanged: () => void;
	} = $props();

	const text = createPageText(channelText);

	let candidates = $state<ChannelCandidate[]>([]);
	let chosenIDs = $state<string[]>([]);
	let isAdding = $state(false);

	const memberExternalIDs = $derived(new Set(members.map((member) => member.externalID)));
	const addable = $derived(candidates.filter((candidate) => !memberExternalIDs.has(candidate.externalID)));

	$effect(() => {
		if (!open) return;
		chosenIDs = [];
		fetchChannelCandidates()
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

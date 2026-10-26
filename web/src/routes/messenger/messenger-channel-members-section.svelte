<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import PersonMultiSelect from '$lib/components/person-multi-select.svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Item from '$lib/components/ui/item';
	import { Separator } from '$lib/components/ui/separator';
	import { Spinner } from '$lib/components/ui/spinner';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelMember, ChannelRole } from '$lib/components/channel/channel-api';
	import { fetchChannelCandidates, type ChannelCandidate } from '$lib/messenger/channel-candidates';
	import { MessengerRefusal, addChannelMembers, removeChannelMember } from '$lib/messenger/messenger-api';
	import { circleChannels } from '$lib/messenger/circle-channels.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { avatarPresenceOf } from '$lib/messenger/member-presence.svelte';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import UserMinusIcon from '@lucide/svelte/icons/user-minus';
	import { toast } from 'svelte-sonner';

	let {
		expanded = $bindable(false),
		channelID,
		members,
		viewerRole,
		onMembersChanged
	}: {
		expanded?: boolean;
		channelID: string;
		members: ChannelMember[];
		viewerRole?: ChannelRole;
		onMembersChanged: () => void;
	} = $props();

	const text = createPageText(channelText);
	const firstShownCount = 5;

	let candidates = $state<ChannelCandidate[]>([]);
	let isLoadingCandidates = $state(false);
	let candidateError = $state('');
	let chosenIDs = $state<string[]>([]);
	let isAdding = $state(false);
	let removing = $state<ChannelMember | null>(null);
	let isRemoving = $state(false);
	let isShowingAll = $state(false);

	const keepingCircleName = $derived(circleChannels.circleNameOf(channelID));
	const memberCountLabel = $derived(text.channelMemberCount.replace('{count}', String(members.length)));
	const memberExternalIDs = $derived(new Set(members.map((member) => member.externalID)));
	const addable = $derived(candidates.filter((candidate) => !memberExternalIDs.has(candidate.externalID)));
	const hiddenCount = $derived(isShowingAll ? 0 : Math.max(0, members.length - firstShownCount));
	const shownMembers = $derived(hiddenCount > 0 ? members.slice(0, firstShownCount) : members);

	$effect(() => {
		if (!expanded) isShowingAll = false;
	});

	$effect(() => {
		if (!expanded || keepingCircleName) return;
		let active = true;
		chosenIDs = [];
		isLoadingCandidates = true;
		candidateError = '';
		fetchChannelCandidates(text.title)
			.then((found) => { if (active) candidates = found; })
			.catch((error: unknown) => { if (active) candidateError = error instanceof Error ? error.message : text.loadPeopleFailed; })
			.finally(() => { if (active) isLoadingCandidates = false; });
		return () => { active = false; };
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

<Item.Root>
	{#snippet child({ props })}
		<button {...props} type="button" aria-expanded={expanded} onclick={() => (expanded = !expanded)}>
			<Item.Content class="text-left">
				<Item.Title>{text.channelMembersTitle}</Item.Title>
			</Item.Content>
			<Item.Actions class="text-muted-foreground h-7 min-w-0 justify-end text-sm">
				<span>{memberCountLabel}</span>
				{#if !expanded}
					<PersonAvatarStack
						people={members.map((member) => ({
							name: member.name,
							seed: member.memberID ?? member.externalID,
							memberID: member.memberID,
							externalID: member.externalID
						}))}
						label={memberCountLabel}
					/>
				{/if}
				<ChevronDownIcon class={['size-4 shrink-0 transition-transform', expanded && 'rotate-180']} />
			</Item.Actions>
		</button>
	{/snippet}
</Item.Root>
{#if expanded}
	<div class="grid gap-2 px-4 pb-3">
		{#if keepingCircleName}
			<p class="text-muted-foreground text-sm">
				{text.channelMembersFollowCircle.replace('{circle}', keepingCircleName)}
			</p>
		{:else}
			<div class="flex items-start gap-2">
				<div class="min-w-0 flex-1">
					<PersonMultiSelect
						selectedIDs={chosenIDs}
						people={addable}
						label={text.addChannelMembers}
						placeholder={text.addMembersPlaceholder}
						onToggle={switchChosen}
						onRemove={switchChosen}
						disabled={isAdding || isLoadingCandidates || Boolean(candidateError)}
					/>
				</div>
				<Button onclick={addChosen} disabled={isAdding || chosenIDs.length === 0}>
					{#if isAdding}<Spinner />{/if}
					{text.addChannelMembers}
				</Button>
			</div>
			{#if candidateError}<p role="alert" class="text-destructive text-sm">{candidateError}</p>{/if}
		{/if}
		<Item.Group aria-label={text.channelMembersTitle}>
			{#each shownMembers as member (member.memberID ?? member.externalID)}
				<Item.Root size="sm" class="px-0">
					<Item.Media>
						<PersonAvatar
							name={member.name}
							seed={member.memberID ?? member.externalID ?? ''}
							memberID={member.memberID ?? ''}
							externalID={member.externalID ?? ''}
							{...avatarPresenceOf(member.memberID, text)}
							class="size-8"
						/>
					</Item.Media>
					<Item.Content class="min-w-0">
						<Item.Title class="truncate">{displayPersonName(member.name) || text.unnamedMember}</Item.Title>
						{#if member.role === 'owner'}<Item.Description>{text.channelOwner}</Item.Description>{/if}
					</Item.Content>
					{#if viewerRole === 'owner' && member.role === 'member'}
						<Item.Actions>
							<Button
								type="button"
								variant="ghost"
								size="icon-sm"
								aria-label={text.removeChannelMember}
								onclick={() => (removing = member)}
							>
								<UserMinusIcon />
							</Button>
						</Item.Actions>
					{/if}
				</Item.Root>
			{/each}
		</Item.Group>
		{#if hiddenCount > 0}
			<Button variant="ghost" size="sm" class="justify-start" onclick={() => (isShowingAll = true)}>
				{text.showAllChannelMembers.replace('{count}', String(hiddenCount))}
			</Button>
		{/if}
	</div>
{/if}
<Separator />

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

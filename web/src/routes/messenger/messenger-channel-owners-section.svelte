<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Item from '$lib/components/ui/item';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import { Spinner } from '$lib/components/ui/spinner';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelMember, ChannelRole } from '$lib/components/channel/channel-api';
	import { MessengerRefusal, addChannelOwner, handOverChannel } from '$lib/messenger/messenger-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { avatarPresenceOf } from '$lib/messenger/member-presence.svelte';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import { toast } from 'svelte-sonner';

	let {
		expanded = $bindable(false),
		channelID,
		members,
		viewerRole,
		onOwnersChanged
	}: {
		expanded?: boolean;
		channelID: string;
		members: ChannelMember[];
		viewerRole?: ChannelRole;
		onOwnersChanged: () => void;
	} = $props();

	const text = createPageText(channelText);

	let newOwnerKey = $state('');
	let successorKey = $state('');
	let confirmingSuccessor = $state<ChannelMember | null>(null);
	let isSubmitting = $state(false);

	const owners = $derived(members.filter((member) => member.role === 'owner'));
	const candidates = $derived(members.filter((member) => member.role !== 'owner'));
	const ownerNames = $derived(owners.map((owner) => displayPersonName(owner.name) || text.unnamedMember).join(', '));

	function keyOf(member: ChannelMember): string {
		return member.memberID ?? member.externalID ?? '';
	}

	function nameOf(member: ChannelMember): string {
		return displayPersonName(member.name) || text.unnamedMember;
	}

	function candidateWithKey(key: string): ChannelMember | undefined {
		return candidates.find((candidate) => keyOf(candidate) === key);
	}

	async function addOwner() {
		const chosen = candidateWithKey(newOwnerKey);
		if (!chosen) return;
		isSubmitting = true;
		try {
			await addChannelOwner(channelID, chosen.externalID ?? '');
			newOwnerKey = '';
		} catch (failure) {
			toast.error(refusalText(failure, text.addChannelOwnerFailed));
		} finally {
			isSubmitting = false;
			onOwnersChanged();
		}
	}

	async function handOver() {
		if (!confirmingSuccessor) return;
		const successor = confirmingSuccessor;
		isSubmitting = true;
		try {
			await handOverChannel(channelID, successor.externalID ?? '');
			successorKey = '';
		} catch (failure) {
			const partly = failure instanceof MessengerRefusal && failure.reason === 'still-an-owner';
			toast.error(partly ? text.ownershipPartlyHandedOver : refusalText(failure, text.handOverChannelFailed));
		} finally {
			confirmingSuccessor = null;
			isSubmitting = false;
			onOwnersChanged();
		}
	}

	function refusalText(failure: unknown, fallback: string): string {
		if (failure instanceof MessengerRefusal && failure.reason === 'not-owner') return text.notChannelOwner;
		return failure instanceof Error ? failure.message : fallback;
	}
</script>

{#snippet memberPicker(label: string, placeholder: string, value: string, choose: (key: string) => void)}
	{@const chosen = candidateWithKey(value)}
	<Select.Root type="single" {value} onValueChange={choose} disabled={isSubmitting || candidates.length === 0}>
		<Select.Trigger class="min-w-0 flex-1" aria-label={label}>
			{chosen ? nameOf(chosen) : placeholder}
		</Select.Trigger>
		<Select.Content>
			{#each candidates as candidate (keyOf(candidate))}
				<Select.Item value={keyOf(candidate)} label={nameOf(candidate)}>{nameOf(candidate)}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/snippet}

<Item.Root>
	{#snippet child({ props })}
		<button {...props} type="button" aria-expanded={expanded} aria-label={`${text.channelOwnersTitle} ${ownerNames}`} onclick={() => (expanded = !expanded)}>
			<Item.Content class="text-left">
				<Item.Title>{text.channelOwnersTitle}</Item.Title>
			</Item.Content>
			<Item.Actions class="text-muted-foreground h-7 min-w-0 justify-end text-sm">
				{#if !expanded}
					<PersonAvatarStack
						people={owners.map((owner) => ({
							name: owner.name,
							seed: owner.memberID ?? owner.externalID,
							memberID: owner.memberID,
							externalID: owner.externalID
						}))}
						label={ownerNames}
					/>
				{/if}
				<ChevronDownIcon class={['size-4 shrink-0 transition-transform', expanded && 'rotate-180']} />
			</Item.Actions>
		</button>
	{/snippet}
</Item.Root>
{#if expanded}
	<div class="grid gap-2 px-4 pb-3">
		{#if viewerRole === 'owner'}
			<p class="text-muted-foreground text-xs font-medium">{text.addChannelOwnerTitle}</p>
			<div class="flex items-center gap-2">
				{@render memberPicker(text.addChannelOwnerTitle, text.addChannelOwnerPlaceholder, newOwnerKey, (key) => (newOwnerKey = key))}
				<Button onclick={addOwner} disabled={isSubmitting || !candidateWithKey(newOwnerKey)}>
					{#if isSubmitting}<Spinner />{/if}
					{text.addChannelOwnerAction}
				</Button>
			</div>
			<p class="text-muted-foreground text-xs font-medium">{text.handOverChannel}</p>
			<div class="flex items-center gap-2">
				{@render memberPicker(text.handOverChannel, text.handOverChannelPlaceholder, successorKey, (key) => (successorKey = key))}
				<Button
					variant="outline"
					onclick={() => (confirmingSuccessor = candidateWithKey(successorKey) ?? null)}
					disabled={isSubmitting || !candidateWithKey(successorKey)}
				>
					{text.handOverChannelAction}
				</Button>
			</div>
		{/if}
		<ul class="grid" aria-label={text.channelOwnersTitle}>
			{#each owners as owner (keyOf(owner))}
				<li class="flex min-w-0 items-center gap-2.5 py-1.5">
					<PersonAvatar
						name={owner.name}
						seed={keyOf(owner)}
						memberID={owner.memberID ?? ''}
						externalID={owner.externalID ?? ''}
						{...avatarPresenceOf(owner.memberID, text)}
						class="size-7"
					/>
					<span class="truncate text-sm font-medium">{nameOf(owner)}</span>
				</li>
			{/each}
		</ul>
	</div>
{/if}
<Separator />

<AlertDialog.Root open={confirmingSuccessor !== null} onOpenChange={(next) => !next && (confirmingSuccessor = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{text.handOverChannelConfirmTitle.replace('{name}', confirmingSuccessor ? nameOf(confirmingSuccessor) : '')}
			</AlertDialog.Title>
			<AlertDialog.Description>{text.handOverChannelConfirmDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={isSubmitting}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action disabled={isSubmitting} onclick={handOver}>{text.handOverChannelAction}</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

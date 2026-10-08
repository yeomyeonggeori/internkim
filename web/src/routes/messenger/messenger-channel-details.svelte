<script lang="ts">
	import CrownIcon from '@lucide/svelte/icons/crown';
	import DoorOpenIcon from '@lucide/svelte/icons/door-open';
	import FileDownIcon from '@lucide/svelte/icons/file-down';
	import HashIcon from '@lucide/svelte/icons/hash';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import LockIcon from '@lucide/svelte/icons/lock';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Avatar from '$lib/components/ui/avatar';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as Item from '$lib/components/ui/item';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import { channelText } from '$lib/i18n/channel-text';
	import { conversationExportText } from '$lib/i18n/conversation-export-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChannelSummary } from '$lib/components/channel/channel-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { MessengerRefusal, deleteChannel } from '$lib/messenger/messenger-api';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		channel,
		openMembers,
		openOwnerAdd,
		openOwnerHandover,
		openExport,
		openLeave,
		onDeleted
	}: {
		open?: boolean;
		channel: ChannelSummary;
		openMembers: () => void;
		openOwnerAdd: () => void;
		openOwnerHandover: () => void;
		openExport: () => void;
		openLeave: () => void;
		onDeleted: (channelID: string) => void;
	} = $props();

	const text = createPageText(channelText);
	const exportText = createPageText(conversationExportText);
	const valueRowClass = 'text-muted-foreground h-7 min-w-0 justify-end text-sm';
	const actionRowClass = 'min-h-7 justify-center text-left';
	const members = $derived(channel.members ?? []);
	const memberCountLabel = $derived(text.channelMemberCount.replace('{count}', String(members.length)));
	const owners = $derived(members.filter((member) => member.role === 'owner'));
	const ownerNames = $derived(owners.map((owner) => displayPersonName(owner.name) || text.unnamedMember).join(', '));
	const amOwner = $derived(channel.myRole === 'owner');

	let isConfirmingDelete = $state(false);
	let isDeleting = $state(false);

	async function remove() {
		isDeleting = true;
		try {
			await deleteChannel(channel.id);
			isConfirmingDelete = false;
			open = false;
			onDeleted(channel.id);
		} catch (failure) {
			isConfirmingDelete = false;
			if (failure instanceof MessengerRefusal && failure.reason === 'not-owner') {
				toast.error(text.notChannelOwner);
			} else {
				toast.error(failure instanceof Error ? failure.message : text.deleteChannelFailed);
			}
		} finally {
			isDeleting = false;
		}
	}

	const shortChannelID = $derived(
		channel.id.length > 14 ? `${channel.id.slice(0, 8)}…${channel.id.slice(-4)}` : channel.id
	);
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="gap-0 p-0 sm:max-w-sm" closeLabel={text.closeChannelDetails}>
		<Sheet.Header class="border-b">
			<Sheet.Title>{text.channelDetails}</Sheet.Title>
		</Sheet.Header>
		<div class="min-h-0 flex-1 overflow-y-auto">
			<div class="flex flex-col items-center gap-3 px-6 py-6 text-center">
				<Avatar.Root class="size-16">
					<Avatar.Fallback>
						{#if channel.isPrivate}<LockIcon class="size-7" />{:else}<HashIcon class="size-7" />{/if}
					</Avatar.Fallback>
				</Avatar.Root>
				<p class="text-lg font-semibold break-all">{channel.name}</p>
				{#if channel.description}
					<p class="text-muted-foreground text-sm break-words whitespace-pre-line">{channel.description}</p>
				{/if}
			</div>

			<div class="grid gap-2 px-4 pb-6">
				<p class="text-muted-foreground px-2 text-xs font-medium">{text.channelDetailsSection}</p>
				<div class="rounded-lg border">
					<Item.Root>
						<Item.Content>
							<Item.Title>{text.channelVisibility}</Item.Title>
						</Item.Content>
						<Item.Actions class={valueRowClass}>
							{channel.isPrivate ? text.channelPrivate : text.channelPublic}
						</Item.Actions>
					</Item.Root>
					<Separator />
					{#if amOwner}
						<Item.Root>
							{#snippet child({ props })}
								<button {...props} type="button" onclick={openOwnerAdd}>
									<Item.Content class="text-left">
										<Item.Title>{text.channelOwner}</Item.Title>
									</Item.Content>
									<Item.Actions class={valueRowClass}>
										<span class="truncate">{ownerNames}</span>
										<CrownIcon class="size-4 shrink-0" />
									</Item.Actions>
								</button>
							{/snippet}
						</Item.Root>
						<Separator />
						<Item.Root>
							{#snippet child({ props })}
								<button {...props} type="button" onclick={openOwnerHandover}>
									<Item.Content class="text-left">
										<Item.Title>{text.handOverChannel}</Item.Title>
									</Item.Content>
									<Item.Actions class={valueRowClass}><CrownIcon class="size-4" /></Item.Actions>
								</button>
							{/snippet}
						</Item.Root>
					{:else}
						<Item.Root>
							<Item.Content>
								<Item.Title>{text.channelOwner}</Item.Title>
							</Item.Content>
							<Item.Actions class={valueRowClass}><span class="truncate">{ownerNames}</span></Item.Actions>
						</Item.Root>
					{/if}
					<Separator />
					<Item.Root>
						{#snippet child({ props })}
							<button {...props} type="button" onclick={openMembers}>
								<Item.Content class="text-left">
									<Item.Title>{text.channelMembersTitle}</Item.Title>
								</Item.Content>
								<Item.Actions class={valueRowClass}>
									<span>{memberCountLabel}</span>
									<PersonAvatarStack
										people={members.map((member) => ({
											name: member.name,
											seed: member.memberID ?? member.externalID,
											memberID: member.memberID,
											externalID: member.externalID
										}))}
										label={memberCountLabel}
									/>
								</Item.Actions>
							</button>
						{/snippet}
					</Item.Root>
					<Separator />
					<Item.Root>
						<Item.Content>
							<Item.Title>{text.channelIDLabel}</Item.Title>
						</Item.Content>
						<Item.Actions class={valueRowClass}>
							<span class="font-mono" title={channel.id}>{shortChannelID}</span>
							<CopyButton text={channel.id} size="icon-sm" tabindex={0}>
								<span class="sr-only">{text.copyChannelID}</span>
							</CopyButton>
						</Item.Actions>
					</Item.Root>
				</div>

				<div class="mt-4 rounded-lg border">
					<Item.Root>
						{#snippet child({ props })}
							<button {...props} type="button" onclick={openExport}>
								<Item.Media variant="icon"><FileDownIcon /></Item.Media>
								<Item.Content class={actionRowClass}>
									<Item.Title>{exportText.exportConversation}</Item.Title>
								</Item.Content>
							</button>
						{/snippet}
					</Item.Root>
					<Separator />
					<Item.Root>
						{#snippet child({ props })}
							<button {...props} type="button" onclick={openLeave}>
								<Item.Media variant="icon"><DoorOpenIcon /></Item.Media>
								<Item.Content class={actionRowClass}>
									<Item.Title>{text.leaveChannel}</Item.Title>
								</Item.Content>
							</button>
						{/snippet}
					</Item.Root>
					{#if amOwner}
						<Separator />
						<Item.Root>
							{#snippet child({ props })}
								<button {...props} type="button" onclick={() => (isConfirmingDelete = true)}>
									<Item.Media variant="icon"><Trash2Icon class="text-destructive" /></Item.Media>
									<Item.Content class={actionRowClass}>
										<Item.Title class="text-destructive">{text.deleteChannel}</Item.Title>
									</Item.Content>
								</button>
							{/snippet}
						</Item.Root>
					{/if}
				</div>
			</div>
		</div>
	</Sheet.Content>
</Sheet.Root>

<AlertDialog.Root bind:open={isConfirmingDelete}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.deleteChannelTitle.replace('{name}', channel.name)}</AlertDialog.Title>
			<AlertDialog.Description>{text.deleteChannelDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={isDeleting}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" disabled={isDeleting} onclick={remove}>
				{text.delete}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

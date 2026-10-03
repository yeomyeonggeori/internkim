<script lang="ts">
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import LockIcon from '@lucide/svelte/icons/lock';
	import PersonMultiSelect from '$lib/components/person-multi-select.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { seatCompanyAdminsAsOwners } from '$lib/messenger/channel-admin-owners';
	import { fetchChannelCandidates, type ChannelCandidate } from '$lib/messenger/channel-candidates';
	import { createChannel, type NewChannel } from '$lib/messenger/messenger-api';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		onCreated
	}: {
		open?: boolean;
		onCreated: (channelID: string) => void;
	} = $props();

	const text = createPageText(channelText);

	let name = $state('');
	let description = $state('');
	let visibility = $state<NewChannel['visibility']>('open');
	let memberIDs = $state<string[]>([]);
	let candidates = $state<ChannelCandidate[]>([]);
	let errorMessage = $state('');
	let isCreating = $state(false);

	const channelName = $derived(name.trim());

	$effect(() => {
		if (!open) return;
		name = '';
		description = '';
		visibility = 'open';
		memberIDs = [];
		errorMessage = '';
		fetchChannelCandidates(text.title)
			.then((found) => (candidates = found))
			.catch(() => (candidates = []));
	});

	function switchMember(memberID: string) {
		memberIDs = memberIDs.includes(memberID)
			? memberIDs.filter((selected) => selected !== memberID)
			: [...memberIDs, memberID];
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		isCreating = true;
		errorMessage = '';
		try {
			const invited = candidates.filter((candidate) => memberIDs.includes(candidate.memberID));
			const created = await createChannel({
				name: channelName,
				description: description.trim() || undefined,
				visibility,
				memberExternalIDs: invited.map((candidate) => candidate.externalID)
			});
			let unseatedAdminNames: string[] = [];
			try {
				unseatedAdminNames = await seatCompanyAdminsAsOwners(created.channel.id);
			} catch (refusal) {
				console.warn('the company administrators were not seated as owners', refusal);
				toast.warning(text.adminOwnersNotSeatedAtAll);
			}
			open = false;
			onCreated(created.channel.id);
			const uninvitedNames = invited
				.filter((candidate) => created.uninvitedExternalIDs.includes(candidate.externalID))
				.map((candidate) => displayPersonName(candidate.name));
			if (uninvitedNames.length > 0) {
				toast.warning(text.inviteFailed.replace('{names}', uninvitedNames.join(', ')));
			}
			if (unseatedAdminNames.length > 0) {
				toast.warning(text.adminOwnersNotSeated.replace('{names}', unseatedAdminNames.join(', ')));
			}
		} catch (failure) {
			errorMessage = failure instanceof Error ? failure.message : text.createChannelFailed;
		} finally {
			isCreating = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{text.newChannel}</Dialog.Title>
		</Dialog.Header>
		<form onsubmit={submit}>
			<Field.Group>
				<Field.Field>
					<Field.Label for="new-channel-name">{text.channelName}</Field.Label>
					<Input
						id="new-channel-name"
						bind:value={name}
						placeholder={text.channelNamePlaceholder}
						autocomplete="off"
						disabled={isCreating}
					/>
				</Field.Field>
				<Field.Field>
					<Field.Label for="new-channel-description">
						{text.channelDescription}
						<span class="text-muted-foreground font-normal">{text.optional}</span>
					</Field.Label>
					<Textarea
						id="new-channel-description"
						bind:value={description}
						placeholder={text.channelDescriptionPlaceholder}
						disabled={isCreating}
					/>
				</Field.Field>
				<Field.Field>
					<Field.Label>{text.channelVisibility}</Field.Label>
					<ToggleGroup.Root
						type="single"
						value={visibility}
						onValueChange={(chosen) => {
							if (chosen === 'open' || chosen === 'private') visibility = chosen;
						}}
						variant="outline"
						disabled={isCreating}
						aria-label={text.channelVisibility}
						class="w-full"
					>
						<ToggleGroup.Item value="open" class="flex-1"><GlobeIcon />{text.channelPublic}</ToggleGroup.Item>
						<ToggleGroup.Item value="private" class="flex-1"><LockIcon />{text.channelPrivate}</ToggleGroup.Item>
					</ToggleGroup.Root>
					<Field.Description>
						{visibility === 'open' ? text.channelPublicHint : text.channelPrivateHint}
					</Field.Description>
				</Field.Field>
				<Field.Field>
					<Field.Label for="new-channel-members">
						{text.channelMembers}
						<span class="text-muted-foreground font-normal">{text.optional}</span>
					</Field.Label>
					<PersonMultiSelect
						id="new-channel-members"
						selectedIDs={memberIDs}
						people={candidates}
						label={text.channelMembers}
						placeholder={text.searchPeople}
						onToggle={switchMember}
						onRemove={switchMember}
						disabled={isCreating}
						side="top"
					/>
				</Field.Field>
				{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
			</Field.Group>
			<Dialog.Footer class="mt-6">
				<Button type="submit" disabled={isCreating || !channelName}>
					{#if isCreating}<Spinner />{/if}
					{text.createChannel}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

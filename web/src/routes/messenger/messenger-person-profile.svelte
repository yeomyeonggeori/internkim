<script lang="ts">
	import MessageCircleIcon from '@lucide/svelte/icons/message-circle';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { MentionPerson } from '$lib/messenger/mention-candidates';
	import { personProfileRecordOf, type PersonProfileRecord } from '$lib/messenger/person-profile-record';
	import OrganizationPersonDetails from '../organization/organization-person-details.svelte';
	import { organizationDirectoryText } from '../organization/text';

	let {
		open = $bindable(false),
		person,
		onMessage
	}: {
		open?: boolean;
		person: MentionPerson | undefined;
		onMessage: (person: MentionPerson) => void;
	} = $props();

	const text = createPageText(channelText);
	const organizationText = createPageText(organizationDirectoryText);
	let profile = $state<PersonProfileRecord | undefined>(undefined);
	let readSequence = 0;

	$effect(() => {
		const wanted = open ? person : undefined;
		const sequence = ++readSequence;
		profile = undefined;
		if (!wanted) return;
		personProfileRecordOf(wanted)
			.then((found) => {
				if (sequence === readSequence) profile = found;
			})
			.catch((failure: unknown) => console.warn('the person profile did not load', { externalID: wanted.externalID, failure }));
	});
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="gap-0 p-0 sm:max-w-sm" closeLabel={text.closePersonProfile}>
		<Sheet.Header class="border-b">
			<Sheet.Title>{text.personProfile}</Sheet.Title>
		</Sheet.Header>
		{#if person}
			<div class="flex min-h-0 flex-col overflow-y-auto overscroll-contain" data-testid="messenger-person-profile">
				<div class="flex flex-col items-center gap-1 px-6 py-8 text-center">
					<PersonAvatar
						class="mb-2 size-24"
						name={person.name}
						externalID={person.externalID}
						memberID={person.memberID ?? ''}
						image={person.avatarURL ?? ''}
					/>
					<p class="text-xl font-semibold break-all">{person.name}</p>
					{#if profile?.record.jobTitle}
						<p class="text-muted-foreground text-sm">{profile.record.jobTitle}</p>
					{/if}
				</div>
				<div class="px-4">
					<Button variant="secondary" class="h-auto w-full flex-col gap-1 py-3" onclick={() => onMessage(person)}>
						<MessageCircleIcon class="size-5" />
						{text.sendDirectMessage}
					</Button>
				</div>
				{#if profile}
					<div class="px-4 py-5">
						<OrganizationPersonDetails
							record={profile.record}
							groups={profile.groups}
							userRecords={profile.userRecords}
							text={organizationText}
						/>
					</div>
				{/if}
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>

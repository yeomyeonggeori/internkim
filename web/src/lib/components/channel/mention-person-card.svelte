<script lang="ts">
	import MemberCard from '$lib/components/member/member-card.svelte';
	import { memberRolesOf, type MemberProfile } from '$lib/components/member/member-role';
	import MentionChip from './mention-chip.svelte';
	import * as HoverCard from '$lib/components/ui/hover-card';
	import type { MentionPerson } from '$lib/messenger/mention-candidates';
	import type { PersonProfileActions } from '$lib/messenger/person-profile';
	import { personProfileRecordOf, type PersonProfileRecord } from '$lib/messenger/person-profile-record';
	import { responsibleMemberIDOf } from '$lib/organization/responsible-member';

	let { text, person, actions }: { text: string; person: MentionPerson; actions: PersonProfileActions } = $props();

	let isOpen = $state(false);
	let profile = $state<PersonProfileRecord | undefined>(undefined);

	$effect(() => {
		if (!isOpen || profile) return;
		const wanted = person;
		personProfileRecordOf(wanted)
			.then((found) => {
				if (wanted === person) profile = found;
			})
			.catch((failure: unknown) => console.warn('the mentioned person profile did not load', { externalID: wanted.externalID, failure }));
	});

	function memberOf(person: MentionPerson, profile: PersonProfileRecord | undefined): MemberProfile {
		const known = { memberID: person.memberID ?? '', externalID: person.externalID, name: person.name, email: '', image: person.avatarURL, roles: [] };
		if (!profile) return known;
		const { record, groups, userRecords, representativeID } = profile;
		const teamRecords = record.groupID ? userRecords.filter((candidate) => candidate.groupID === record.groupID) : [];
		return {
			...known,
			memberID: record.memberID,
			email: record.email,
			image: record.image || person.avatarURL,
			jobTitle: record.jobTitle,
			team: groups.find((group) => group.id === record.groupID)?.name,
			phoneNumber: record.phoneNumber,
			roles: memberRolesOf(record.memberID, representativeID, responsibleMemberIDOf(teamRecords))
		};
	}
</script>

<HoverCard.Root bind:open={isOpen} openDelay={300} closeDelay={100}>
	<HoverCard.Trigger>
		{#snippet child({ props })}
			<MentionChip {...props} {text} onclick={() => actions.show(person)} />
		{/snippet}
	</HoverCard.Trigger>
	<HoverCard.Content class="w-56 overflow-hidden p-0" align="start">
		<MemberCard
			member={memberOf(person, profile)}
			showsTeam
			onSelect={() => actions.show(person)}
			onMessage={() => actions.message(person)}
			class="rounded-none border-0"
		/>
	</HoverCard.Content>
</HoverCard.Root>

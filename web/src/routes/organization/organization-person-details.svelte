<script lang="ts">
	import { displayPersonName } from '$lib/person-name.svelte';
	import * as Item from '$lib/components/ui/item';
	import BriefcaseBusinessIcon from '@lucide/svelte/icons/briefcase-business';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import UserRoundIcon from '@lucide/svelte/icons/user-round';
	import type { OrgGroup, UserRecord } from '../../lib/organization/types';
	import type { organizationDirectoryText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	let {
		record,
		groups,
		userRecords,
		text
	}: {
		record: UserRecord;
		groups: OrgGroup[];
		userRecords: UserRecord[];
		text: PageText<typeof organizationDirectoryText>;
	} = $props();

	function groupName(groupID: string | undefined): string {
		const normalizedGroupID = groupID?.trim() ?? '';
		if (!normalizedGroupID) return text.unassignedTeam;
		return groups.find((group) => group.id === normalizedGroupID)?.name ?? normalizedGroupID;
	}

	function directSupervisorLabel(person: UserRecord): string {
		const supervisorID = person.supervisorID?.trim() ?? '';
		if (!supervisorID) return text.noSupervisor;
		const supervisor = userRecords.find((candidate) => candidate.memberID === supervisorID);
		if (!supervisor) return text.noSupervisor;
		return `${displayPersonName(supervisor.name || supervisor.email)} · ${supervisor.jobTitle || text.noTitle}`;
	}
</script>

<Item.Group class="gap-2">
	<Item.Root variant="muted" size="sm">
		<Item.Media variant="icon"><MailIcon /></Item.Media>
		<Item.Content>
			<Item.Description>{text.email}</Item.Description>
			<Item.Title class="w-full truncate">{record.email}</Item.Title>
		</Item.Content>
	</Item.Root>
	<Item.Root variant="muted" size="sm">
		<Item.Media variant="icon"><PhoneIcon /></Item.Media>
		<Item.Content>
			<Item.Description>{text.phoneNumber}</Item.Description>
			<Item.Title class="w-full truncate">{record.phoneNumber || text.notProvided}</Item.Title>
		</Item.Content>
	</Item.Root>
	<Item.Root variant="muted" size="sm">
		<Item.Media variant="icon"><ComponentIcon /></Item.Media>
		<Item.Content>
			<Item.Description>{text.primaryOrganization}</Item.Description>
			<Item.Title class="w-full truncate">{groupName(record.groupID)}</Item.Title>
		</Item.Content>
	</Item.Root>
	<Item.Root variant="muted" size="sm">
		<Item.Media variant="icon"><BriefcaseBusinessIcon /></Item.Media>
		<Item.Content>
			<Item.Description>{text.position}</Item.Description>
			<Item.Title class="w-full truncate">{record.jobTitle || text.noTitle}</Item.Title>
		</Item.Content>
	</Item.Root>
	<Item.Root variant="muted" size="sm">
		<Item.Media variant="icon"><UserRoundIcon /></Item.Media>
		<Item.Content>
			<Item.Description>{text.directSupervisor}</Item.Description>
			<Item.Title class="w-full truncate">{directSupervisorLabel(record)}</Item.Title>
		</Item.Content>
	</Item.Root>
	<Item.Root variant="muted" size="sm">
		<Item.Media variant="icon"><TimerIcon /></Item.Media>
		<Item.Content>
			<Item.Description>{text.hireDate}</Item.Description>
			<Item.Title class="w-full truncate">{record.hireDate || text.notProvided}</Item.Title>
		</Item.Content>
	</Item.Root>
</Item.Group>

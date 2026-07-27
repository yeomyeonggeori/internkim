<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import BriefcaseBusinessIcon from '@lucide/svelte/icons/briefcase-business';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import UserRoundIcon from '@lucide/svelte/icons/user-round';
	import type { AdminPageText } from '../admin/admin-types';
	import OrganizationProfileFields from '../admin/organization-profile-fields.svelte';
	import type { OrgGroup, UserRecord } from '../../lib/organization/types';
	import type { organizationDirectoryText } from './text';

	type OrganizationPersonDetailPanelProps = {
		record: UserRecord | undefined;
		canEditOwnPhoneNumber?: boolean;
		isSavingOwnPhoneNumber?: boolean;
		onSaveOwnPhoneNumber?: (phoneNumber: string) => void | Promise<void>;
		groups: OrgGroup[];
		text: typeof organizationDirectoryText.ko;
		canEdit?: boolean;
		isEditing?: boolean;
		editingRecord?: UserRecord;
		userRecords?: UserRecord[];
		adminText?: AdminPageText;
		isSaving?: boolean;
		hasInvalidSupervisor?: boolean;
		onEdit?: () => void;
		onSave?: () => void | Promise<void>;
		onCancel?: () => void;
	};

	let {
		record,
		groups,
		text,
		canEditOwnPhoneNumber = false,
		isSavingOwnPhoneNumber = false,
		onSaveOwnPhoneNumber = () => {},
		canEdit = false,
		isEditing = false,
		editingRecord,
		userRecords = [],
		adminText,
		isSaving = false,
		hasInvalidSupervisor = false,
		onEdit = () => {},
		onSave = () => {},
		onCancel = () => {}
	}: OrganizationPersonDetailPanelProps = $props();

	let localEditingRecord = $state<UserRecord | undefined>(undefined);
	let ownPhoneNumberDraft = $state<string | undefined>(undefined);

	$effect(() => {
		localEditingRecord = editingRecord;
	});

	function groupName(groupID: string | undefined): string {
		const normalizedGroupID = groupID?.trim() ?? '';
		if (!normalizedGroupID) return text.unassignedTeam;
		return groups.find((group) => group.id === normalizedGroupID)?.name ?? normalizedGroupID;
	}

	function personLabel(person: UserRecord): string {
		return person.name || person.email;
	}

	function personTitle(person: UserRecord): string {
		return person.jobTitle || text.noTitle;
	}

	function directSupervisorLabel(person: UserRecord): string {
		const supervisorID = person.supervisorID?.trim() ?? '';
		if (!supervisorID) return text.noSupervisor;
		const supervisor = userRecords.find((candidate) => candidate.userID === supervisorID);
		if (!supervisor) return text.noSupervisor;
		return `${personLabel(supervisor)} · ${personTitle(supervisor)}`;
	}
</script>

{#if record}
	<div class="grid min-h-0 grid-rows-[auto_minmax(0,1fr)_auto]" data-testid="organization-person-detail-panel">
		<Item.Root class="px-6 py-5">
			<Item.Media>
				<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-14" />
			</Item.Media>
			<Item.Content>
				<Item.Title class="w-full truncate text-lg">{personLabel(record)}</Item.Title>
				<Item.Description>{record.jobTitle || text.noTitle}</Item.Description>
			</Item.Content>
		</Item.Root>

		<div class="min-h-0 overflow-y-auto overscroll-contain px-6 py-5">
			{#if isEditing && localEditingRecord && adminText}
				<div class="grid gap-4" data-testid={`organization-profile-${localEditingRecord.userID}`}>
					<Item.Root variant="muted" size="sm">
						<Item.Media variant="icon"><MailIcon /></Item.Media>
						<Item.Content>
							<Item.Description>{text.email}</Item.Description>
							<Item.Title class="w-full truncate">{record.email}</Item.Title>
						</Item.Content>
					</Item.Root>
					<OrganizationProfileFields
						bind:record={localEditingRecord}
						{userRecords}
						{groups}
						text={adminText}
						{isSaving}
						layout="stacked"
					/>
				</div>
			{:else}
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
			{/if}
		</div>

		{#if canEdit}
			<div class="grid gap-2 border-t px-6 py-4">
				{#if isEditing && adminText}
					<div class="grid grid-cols-2 gap-2">
						<Button type="button" variant="outline" disabled={isSaving} onclick={onCancel}>
							{adminText.organization.cancel}
						</Button>
						<Button type="button" disabled={isSaving || hasInvalidSupervisor} onclick={onSave}>
							{adminText.organization.save}
						</Button>
					</div>
				{:else}
					<Button type="button" onclick={onEdit}>{text.editPerson}</Button>
				{/if}
			</div>
		{/if}
	</div>
{/if}

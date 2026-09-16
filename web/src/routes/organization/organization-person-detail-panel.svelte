<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import BriefcaseBusinessIcon from '@lucide/svelte/icons/briefcase-business';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import UserRoundIcon from '@lucide/svelte/icons/user-round';
	import type { AdminPageText } from '../admin/admin-types';
	import OrganizationProfileFields from '../admin/organization-profile-fields.svelte';
	import type { OrgGroup, UserRecord } from '../../lib/organization/types';
	import { dataRoomClearanceLabel, dataRoomClearanceOf } from './organization-clearance-model';
	import type { organizationDirectoryText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type OrganizationPersonDetailPanelProps = {
		record: UserRecord | undefined;
		canEditOwnProfile?: boolean;
		isSavingOwnProfile?: boolean;
		onSaveOwnProfile?: (profile: { phoneNumber: string; hireDate: string }) => void | Promise<void>;
		groups: OrgGroup[];
		text: PageText<typeof organizationDirectoryText>;
		canEdit?: boolean;
		isEditing?: boolean;
		editingRecord?: UserRecord;
		userRecords?: UserRecord[];
		adminText?: AdminPageText;
		isSaving?: boolean;
		hasInvalidSupervisor?: boolean;
		canEditClearance?: boolean;
		clearanceChoices?: number[];
		onEdit?: () => void;
		onSave?: () => void | Promise<void>;
		onCancel?: () => void;
	};

	let {
		record,
		groups,
		text,
		canEditOwnProfile = false,
		isSavingOwnProfile = false,
		onSaveOwnProfile = () => {},
		canEdit = false,
		isEditing = false,
		editingRecord,
		userRecords = [],
		adminText,
		isSaving = false,
		hasInvalidSupervisor = false,
		canEditClearance = false,
		clearanceChoices = [],
		onEdit = () => {},
		onSave = () => {},
		onCancel = () => {}
	}: OrganizationPersonDetailPanelProps = $props();

	let localEditingRecord = $state<UserRecord | undefined>(undefined);
	let ownProfileDraft = $state<{ phoneNumber: string; hireDate: string } | undefined>(undefined);

	$effect(() => {
		localEditingRecord = editingRecord;
	});

	function groupName(groupID: string | undefined): string {
		const normalizedGroupID = groupID?.trim() ?? '';
		if (!normalizedGroupID) return text.unassignedTeam;
		return groups.find((group) => group.id === normalizedGroupID)?.name ?? normalizedGroupID;
	}

	function personLabel(person: UserRecord): string {
		return displayPersonName(person.name || person.email);
	}

	function personTitle(person: UserRecord): string {
		return person.jobTitle || text.noTitle;
	}

	function clearanceLabel(person: UserRecord): string {
		return dataRoomClearanceLabel(dataRoomClearanceOf(person), text);
	}

	function selectClearanceValue(value: string): void {
		if (!localEditingRecord) return;
		localEditingRecord.clearance = Number(value);
	}

	function directSupervisorLabel(person: UserRecord): string {
		const supervisorID = person.supervisorID?.trim() ?? '';
		if (!supervisorID) return text.noSupervisor;
		const supervisor = userRecords.find((candidate) => candidate.memberID === supervisorID);
		if (!supervisor) return text.noSupervisor;
		return `${personLabel(supervisor)} · ${personTitle(supervisor)}`;
	}
</script>

{#if record}
	<div class="grid min-h-0 grid-rows-[auto_minmax(0,1fr)_auto]" data-testid="organization-person-detail-panel">
		<Item.Root class="px-6 py-5">
			<Item.Media>
				<PersonAvatar name={displayPersonName(record.name)} email={record.email} seed={record.memberID} image={record.image ?? ''} class="size-14" />
			</Item.Media>
			<Item.Content>
				<Item.Title class="w-full truncate text-lg">{personLabel(record)}</Item.Title>
				<Item.Description>{record.jobTitle || text.noTitle}</Item.Description>
			</Item.Content>
		</Item.Root>

		<div class="min-h-0 overflow-y-auto overscroll-contain px-6 py-5">
			{#if !canEdit && canEditOwnProfile && ownProfileDraft}
				<div class="grid gap-4" data-testid={`organization-own-profile-${record.memberID}`}>
					<div class="grid gap-2">
						<Label for={`organization-own-phone-${record.memberID}`} class="text-xs">{text.phoneNumber}</Label>
						<Input
							id={`organization-own-phone-${record.memberID}`}
							bind:value={ownProfileDraft.phoneNumber}
							autocomplete="tel"
							inputmode="tel"
							disabled={isSavingOwnProfile}
						/>
					</div>
					<div class="grid gap-2">
						<Label for={`organization-own-hire-date-${record.memberID}`} class="text-xs">{text.hireDate}</Label>
						<Input
							id={`organization-own-hire-date-${record.memberID}`}
							type="date"
							bind:value={ownProfileDraft.hireDate}
							disabled={isSavingOwnProfile}
						/>
					</div>
				</div>
			{:else if isEditing && localEditingRecord && adminText}
				<div class="grid gap-4" data-testid={`organization-profile-${localEditingRecord.memberID}`}>
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
					{#if canEditClearance}
						<div class="grid min-h-14 content-start gap-1.5">
							<Label class="text-xs">{text.dataRoomClearance}</Label>
							<Select.Root
								type="single"
								value={String(dataRoomClearanceOf(localEditingRecord))}
								onValueChange={selectClearanceValue}
								disabled={isSaving}
							>
								<Select.Trigger class="w-full font-normal" aria-label={text.dataRoomClearance} data-testid="organization-clearance-select">
									{clearanceLabel(localEditingRecord)}
								</Select.Trigger>
								<Select.Content>
									{#each clearanceChoices as choice (choice)}
										<Select.Item value={String(choice)} label={dataRoomClearanceLabel(choice, text)}>{dataRoomClearanceLabel(choice, text)}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
					{/if}
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
					<Item.Root variant="muted" size="sm">
						<Item.Media variant="icon"><ShieldIcon /></Item.Media>
						<Item.Content>
							<Item.Description>{text.dataRoomClearance}</Item.Description>
							<Item.Title class="w-full truncate" data-testid="organization-clearance">{clearanceLabel(record)}</Item.Title>
						</Item.Content>
					</Item.Root>
				</Item.Group>
			{/if}
		</div>

		{#if canEdit || canEditOwnProfile}
			<div class="grid gap-2 border-t px-6 py-4">
				{#if !canEdit && canEditOwnProfile}
					{#if ownProfileDraft === undefined}
						<Button
							type="button"
							onclick={() => (ownProfileDraft = { phoneNumber: record.phoneNumber ?? '', hireDate: record.hireDate ?? '' })}
						>
							{text.editPerson}
						</Button>
					{:else}
						<div class="grid grid-cols-2 gap-2">
							<Button type="button" variant="outline" disabled={isSavingOwnProfile} onclick={() => (ownProfileDraft = undefined)}>
								{adminText?.organization.cancel}
							</Button>
							<Button
								type="button"
								disabled={isSavingOwnProfile}
								onclick={async () => {
									if (ownProfileDraft) await onSaveOwnProfile(ownProfileDraft);
									ownProfileDraft = undefined;
								}}
							>
								{adminText?.organization.save}
							</Button>
						</div>
					{/if}
				{:else if isEditing && adminText}
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

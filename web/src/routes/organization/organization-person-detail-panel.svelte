<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Button } from '$lib/components/ui/button';
	import DatePicker from '$lib/components/date-picker.svelte';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { Label } from '$lib/components/ui/label';
	import MailIcon from '@lucide/svelte/icons/mail';
	import type { AdminPageText } from '../admin/admin-types';
	import OrganizationPersonDetails from './organization-person-details.svelte';
	import OrganizationProfileFields from '../admin/organization-profile-fields.svelte';
	import type { OrgGroup, UserRecord } from '../../lib/organization/types';
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
		onEdit = () => {},
		onSave = () => {},
		onCancel = () => {}
	}: OrganizationPersonDetailPanelProps = $props();

	let localEditingRecord = $state<UserRecord | undefined>(undefined);
	let ownProfileDraft = $state<{ phoneNumber: string; hireDate: string } | undefined>(undefined);

	$effect(() => {
		localEditingRecord = editingRecord;
	});

	function personLabel(person: UserRecord): string {
		return displayPersonName(person.name || person.email);
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
						<DatePicker
							id={`organization-own-hire-date-${record.memberID}`}
							bind:value={ownProfileDraft.hireDate}
							disabled={isSavingOwnProfile}
							clearable
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
				</div>
			{:else}
				<OrganizationPersonDetails {record} {groups} {userRecords} {text} />
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

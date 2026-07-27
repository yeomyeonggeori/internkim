<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import XIcon from '@lucide/svelte/icons/x';
	import type { AdminPageText } from '../admin/admin-types';
	import OrganizationProfileFields from '../admin/organization-profile-fields.svelte';
	import type { OrgGroup, UserRecord } from '../../lib/organization/types';
	import type { organizationDirectoryText } from './text';

	type OrganizationPersonDetailPanelVariant = 'side' | 'compact' | 'sheet';

	type OrganizationPersonDetailPanelProps = {
		record: UserRecord | undefined;
		groups: OrgGroup[];
		text: typeof organizationDirectoryText.ko;
		clearSelection: () => void;
		variant?: OrganizationPersonDetailPanelVariant;
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
		clearSelection,
		variant = 'side',
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

	const isSheet = $derived(variant === 'sheet');
	const isCompact = $derived(variant !== 'side');

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
	<aside
		class={[
			isSheet ? 'max-h-[85svh] min-h-0 overflow-y-auto overscroll-contain bg-transparent' : 'rounded-lg border bg-card shadow-sm',
			!isSheet && (isCompact ? 'self-start overflow-hidden' : 'h-full min-h-0 overflow-y-auto')
		]}
		data-testid="organization-person-detail-panel"
	>
		<div class={['grid', isSheet ? 'gap-4 p-4 pb-[calc(1rem+env(safe-area-inset-bottom))]' : isCompact ? 'gap-4 p-4' : 'gap-5 p-5']}>
			<div class={['flex items-center justify-between gap-3', isSheet ? 'sticky top-0 z-10 -mx-4 -mt-4 border-b bg-popover px-4 py-3' : '']}>
				<h2 class="text-base font-semibold">{text.personDetail}</h2>
				<Button variant="ghost" size="icon" aria-label={text.closeDetail} onclick={clearSelection}>
					<XIcon class="size-4" />
				</Button>
			</div>

			<div class="flex min-w-0 items-center gap-4">
				<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class={isCompact ? 'size-11' : 'size-14'} />
				<div class="min-w-0">
					<h3 class={['truncate font-semibold', isCompact ? 'text-lg' : 'text-xl']}>{personLabel(record)}</h3>
					<p class="truncate text-sm text-muted-foreground">{record.jobTitle || text.noTitle}</p>
				</div>
			</div>

			<section class={['grid border-t', isCompact ? 'gap-3 pt-4' : 'gap-4 pt-5']}>
				<h3 class="text-sm font-semibold">{text.basicInformation}</h3>
				<dl class="grid gap-0 text-sm">
					<div class={['grid border-b py-3', isCompact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'grid-cols-[86px_minmax(0,1fr)]']}>
						<dt class="text-muted-foreground">{text.email}</dt>
						<dd class="truncate">{record.email}</dd>
					</div>
					<div class={['grid border-b py-3', isCompact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'grid-cols-[86px_minmax(0,1fr)]']}>
						<dt class="text-muted-foreground">{text.phoneNumber}</dt>
						<dd class="truncate">{record.phoneNumber || text.notProvided}</dd>
					</div>
					<div class={['grid border-b py-3', isCompact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'grid-cols-[86px_minmax(0,1fr)]']}>
						<dt class="text-muted-foreground">{text.primaryOrganization}</dt>
						<dd>{groupName(record.groupID)}</dd>
					</div>
					<div class={['grid border-b py-3', isCompact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'grid-cols-[86px_minmax(0,1fr)]']}>
						<dt class="text-muted-foreground">{text.position}</dt>
						<dd>{record.jobTitle || text.noTitle}</dd>
					</div>
					<div class={['grid border-b py-3', isCompact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'grid-cols-[86px_minmax(0,1fr)]']}>
						<dt class="text-muted-foreground">{text.directSupervisor}</dt>
						<dd class="truncate">{directSupervisorLabel(record)}</dd>
					</div>
					<div class={['grid py-3', isCompact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'grid-cols-[86px_minmax(0,1fr)]']}>
						<dt class="text-muted-foreground">{text.hireDate}</dt>
						<dd>{record.hireDate || text.notProvided}</dd>
					</div>
				</dl>
			</section>

			{#if isEditing && localEditingRecord && adminText}
				<section class="grid gap-3 border-t pt-4" data-testid={`organization-profile-${localEditingRecord.userID}`}>
					<h3 class="text-sm font-semibold">{adminText.organization.editMode}</h3>
					<OrganizationProfileFields
						bind:record={localEditingRecord}
						{userRecords}
						{groups}
						text={adminText}
						{isSaving}
						layout="stacked"
					/>
					<div class="flex justify-end gap-2">
						<Button type="button" size="sm" variant="outline" disabled={isSaving} onclick={onCancel}>
							{adminText.organization.cancel}
						</Button>
						<Button type="button" size="sm" disabled={isSaving || hasInvalidSupervisor} onclick={onSave}>
							{adminText.organization.save}
						</Button>
					</div>
				</section>
			{:else if canEdit}
				<div class="border-t pt-4">
					<Button type="button" class="w-full" onclick={onEdit}>
						{text.editPerson}
					</Button>
				</div>
			{/if}
		</div>
	</aside>
{/if}

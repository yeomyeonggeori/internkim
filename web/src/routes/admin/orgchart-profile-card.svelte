<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import type { AdminPageText, OrgGroup, UserRecord } from './admin-types';
	import OrgchartProfileFields from './orgchart-profile-fields.svelte';

	type OrgchartProfileCardProps = {
		record: UserRecord;
		userRecords: UserRecord[];
		groups: OrgGroup[];
		text: AdminPageText;
		canEdit: boolean;
		isEditing: boolean;
		isSaving: boolean;
		hasInvalidSupervisor: boolean;
		onEdit: () => void;
		onSave: () => void | Promise<void>;
		onCancel: () => void;
	};

	let {
		record = $bindable(),
		userRecords,
		groups,
		text,
		canEdit,
		isEditing,
		isSaving,
		hasInvalidSupervisor,
		onEdit,
		onSave,
		onCancel
	}: OrgchartProfileCardProps = $props();

	function groupName(groupID: string) {
		return groups.find((group) => group.id === groupID)?.name ?? '';
	}

	function personLabel(userRecord: UserRecord) {
		return userRecord.name || userRecord.email;
	}
</script>

<div class="rounded-lg border bg-card px-3.5 py-3 shadow-sm" data-testid={`orgchart-profile-${record.userID}`}>
	<div class="flex min-w-0 items-center gap-3">
		<PersonAvatar name={record.name} email={record.email} class="size-9" />
		<div class="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-2 gap-y-0.5">
			<span class="truncate text-sm font-medium">{personLabel(record)}</span>
			{#if record.jobTitle}
				<span class="text-muted-foreground truncate text-xs">{record.jobTitle}</span>
			{/if}
		</div>
		{#if record.primaryGroupID && groupName(record.primaryGroupID)}
			<Badge variant="secondary" class="shrink-0">{groupName(record.primaryGroupID)}</Badge>
		{/if}
		{#if canEdit && !isEditing}
			<Button type="button" size="sm" variant="outline" disabled={isSaving} onclick={onEdit}>
				{text.orgchart.editMode}
			</Button>
		{/if}
	</div>

	{#if isEditing}
		<OrgchartProfileFields
			bind:record
			{userRecords}
			{groups}
			{text}
			{isSaving}
		/>
		<div class="mt-2 flex justify-end gap-2 pt-1">
			<Button type="button" size="sm" variant="outline" disabled={isSaving} onclick={onCancel}>
				{text.orgchart.cancel}
			</Button>
			<Button type="button" size="sm" disabled={isSaving || hasInvalidSupervisor} onclick={onSave}>
				{text.orgchart.save}
			</Button>
		</div>
	{/if}
</div>

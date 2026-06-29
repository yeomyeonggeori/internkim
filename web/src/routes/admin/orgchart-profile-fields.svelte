<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import type { AdminPageText, OrgchartEmploymentStatus, OrgGroup, UserRecord } from './admin-types';
	import { orgchartEmploymentStatuses } from './orgchart-profile-model';

	type OrgchartProfileFieldsProps = {
		record: UserRecord;
		userRecords: UserRecord[];
		groups: OrgGroup[];
		text: AdminPageText;
		isSaving: boolean;
		hasInvalidPositionLevel: boolean;
		projectSuggestions: string[];
	};

	let {
		record = $bindable(),
		userRecords,
		groups,
		text,
		isSaving,
		hasInvalidPositionLevel,
		projectSuggestions
	}: OrgchartProfileFieldsProps = $props();

	const noSelectionValue = '__none__';
	const visibleValue = 'visible';
	const hiddenValue = 'hidden';
	const fieldClass = 'grid min-h-14 content-start gap-1.5';
	const controlClass = 'h-8 text-sm';
	const selectTriggerClass = 'w-full font-normal';
	const groupMembershipTriggerClass = 'h-8 w-full justify-between px-2.5 text-sm font-normal';
	const tagsInputClass = 'min-h-8 py-0.5 text-sm [&>input]:h-6';

	function groupName(groupID: string) {
		return groups.find((group) => group.id === groupID)?.name ?? '';
	}

	function personLabel(userRecord: UserRecord) {
		return userRecord.name || userRecord.email;
	}

	function employmentStatusLabel(status: OrgchartEmploymentStatus) {
		return text.orgchart.employmentStatusLabels[status];
	}

	function primaryGroupLabel() {
		return record.primaryGroupID ? groupName(record.primaryGroupID) || text.orgchart.none : text.orgchart.none;
	}

	function groupMembershipLabel() {
		const selectedGroupNames = groups
			.filter((group) => isGroupMember(group.id))
			.map((group) => group.name);
		return selectedGroupNames.length > 0 ? selectedGroupNames.join(', ') : text.orgchart.none;
	}

	function supervisorLabel() {
		const supervisor = userRecords.find((candidate) => candidate.userID === record.supervisorID);
		return supervisor ? personLabel(supervisor) : text.orgchart.none;
	}

	function visibilityLabel() {
		return record.isOrgchartVisible === false ? text.orgchart.hidden : text.orgchart.visible;
	}

	function selectPrimaryGroup(groupID: string) {
		record.primaryGroupID = groupID;
		record.group = groupID;
		record.groupIDs = groupID ? groupIDsWithPrimary(groupID) : record.groupIDs ?? [];
	}

	function selectPrimaryGroupValue(groupID: string) {
		selectPrimaryGroup(groupID === noSelectionValue ? '' : groupID);
	}

	function groupIDsWithPrimary(groupID: string) {
		return [...new Set([groupID, ...(record.groupIDs ?? [])].filter(Boolean))];
	}

	function selectSupervisorValue(supervisorID: string) {
		record.supervisorID = supervisorID === noSelectionValue ? '' : supervisorID;
	}

	function selectVisibility(value: string) {
		record.isOrgchartVisible = value !== hiddenValue;
	}

	function isGroupMember(groupID: string) {
		return (record.groupIDs ?? []).includes(groupID);
	}

	function toggleGroupMembership(groupID: string) {
		const groupIDs = record.groupIDs ?? [];
		if (groupIDs.includes(groupID)) {
			record.groupIDs = groupIDs.filter((candidate) => candidate !== groupID);
			if (record.primaryGroupID === groupID) {
				record.primaryGroupID = '';
				record.group = '';
			}
			return;
		}
		record.groupIDs = [...groupIDs, groupID];
	}
</script>

<div class="mt-3 grid gap-3 border-t pt-3 sm:grid-cols-2 lg:grid-cols-3">
	<label class={fieldClass}>
		<Label class="text-xs">{text.orgchart.jobTitle}</Label>
		<Input class={controlClass} bind:value={record.jobTitle} placeholder={text.orgchart.jobTitlePlaceholder} autocomplete="off" disabled={isSaving} />
	</label>
	<label class={fieldClass}>
		<Label class="text-xs">{text.orgchart.positionLevel}</Label>
		<Input
			type="number"
			min="1"
			step="1"
			class={controlClass}
			bind:value={record.positionLevel}
			aria-invalid={hasInvalidPositionLevel}
			autocomplete="off"
			disabled={isSaving}
		/>
		{#if hasInvalidPositionLevel}
			<span class="text-xs text-destructive">{text.orgchart.positionLevelError}</span>
		{/if}
	</label>
	<div class={fieldClass}>
		<Label class="text-xs">{text.orgchart.primaryGroup}</Label>
		<Select.Root type="single" value={record.primaryGroupID || noSelectionValue} onValueChange={selectPrimaryGroupValue} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.orgchart.primaryGroup}>
				{primaryGroupLabel()}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={noSelectionValue} label={text.orgchart.none}>{text.orgchart.none}</Select.Item>
				{#each groups as group (group.id)}
					<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<div class={fieldClass}>
		<Label class="text-xs">{text.orgchart.groupMemberships}</Label>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button {...props} type="button" variant="outline" size="sm" class={groupMembershipTriggerClass} disabled={isSaving} aria-label={text.orgchart.groupMemberships}>
						<span class="truncate">{groupMembershipLabel()}</span>
						<ChevronDownIcon class="text-muted-foreground size-4 shrink-0" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="max-h-56 w-(--bits-dropdown-menu-anchor-width) min-w-44">
				{#if groups.length === 0}
					<DropdownMenu.Item disabled>{text.orgchart.none}</DropdownMenu.Item>
				{:else}
					{#each groups as group (group.id)}
						<DropdownMenu.CheckboxItem checked={isGroupMember(group.id)} disabled={isSaving} onclick={() => toggleGroupMembership(group.id)}>
							{group.name}
						</DropdownMenu.CheckboxItem>
					{/each}
				{/if}
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>
	<div class={fieldClass}>
		<Label class="text-xs">{text.orgchart.supervisor}</Label>
		<Select.Root type="single" value={record.supervisorID || noSelectionValue} onValueChange={selectSupervisorValue} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.orgchart.supervisor}>
				{supervisorLabel()}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={noSelectionValue} label={text.orgchart.none}>{text.orgchart.none}</Select.Item>
				{#each userRecords.filter((candidate) => candidate.userID !== record.userID) as option (option.userID)}
					<Select.Item value={option.userID} label={personLabel(option)}>{personLabel(option)}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<label class={fieldClass}>
		<Label class="text-xs">{text.orgchart.projects}</Label>
		<TagsInput
			bind:value={record.projectIDs}
			aria-label={text.orgchart.projects}
			placeholder={text.orgchart.projectsPlaceholder}
			suggestions={projectSuggestions}
			disabled={isSaving}
			class={tagsInputClass}
		/>
	</label>
	<label class={fieldClass}>
		<Label class="text-xs">{text.orgchart.teamRole}</Label>
		<Input class={controlClass} bind:value={record.teamRole} placeholder={text.orgchart.teamRolePlaceholder} autocomplete="off" disabled={isSaving} />
	</label>
	<div class={fieldClass}>
		<Label class="text-xs">{text.orgchart.employmentStatus}</Label>
		<Select.Root type="single" bind:value={record.employmentStatus} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.orgchart.employmentStatus}>
				{employmentStatusLabel(record.employmentStatus ?? 'active')}
			</Select.Trigger>
			<Select.Content>
				{#each orgchartEmploymentStatuses as status}
					<Select.Item value={status} label={employmentStatusLabel(status)}>{employmentStatusLabel(status)}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<div class={fieldClass}>
		<Label class="text-xs">{text.orgchart.visibility}</Label>
		<Select.Root type="single" value={record.isOrgchartVisible === false ? hiddenValue : visibleValue} onValueChange={selectVisibility} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.orgchart.visibility}>
				{visibilityLabel()}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={visibleValue} label={text.orgchart.visible}>{text.orgchart.visible}</Select.Item>
				<Select.Item value={hiddenValue} label={text.orgchart.hidden}>{text.orgchart.hidden}</Select.Item>
			</Select.Content>
		</Select.Root>
	</div>
</div>

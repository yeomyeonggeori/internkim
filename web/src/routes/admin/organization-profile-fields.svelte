<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import type { OrgGroup, UserRecord } from '../../lib/organization/types';
	import type { AdminPageText } from './admin-types';
	import { supervisorCandidatesForRecord } from './organization-tree';

	type OrganizationProfileFieldsProps = {
		record: UserRecord;
		userRecords: UserRecord[];
		groups: OrgGroup[];
		text: AdminPageText;
		isSaving: boolean;
		layout?: 'default' | 'stacked';
	};

	let {
		record = $bindable(),
		userRecords,
		groups,
		text,
		isSaving,
		layout = 'default'
	}: OrganizationProfileFieldsProps = $props();

	const noSelectionValue = '__none__';
	const fieldClass = 'grid min-h-14 content-start gap-1.5';
	const controlClass = 'h-8 text-sm';
	const selectTriggerClass = 'w-full font-normal';

	function groupName(groupID: string) {
		return groups.find((group) => group.id === groupID)?.name ?? '';
	}

	function personLabel(userRecord: UserRecord) {
		return userRecord.name || userRecord.email;
	}

	function organizationLabel() {
		return record.groupID ? groupName(record.groupID) || text.organization.none : text.organization.none;
	}

	function supervisorLabel() {
		const supervisor = userRecords.find((candidate) => candidate.userID === record.supervisorID);
		return supervisor ? personLabel(supervisor) : text.organization.none;
	}

	function selectOrganizationValue(groupID: string) {
		record.groupID = groupID === noSelectionValue ? '' : groupID;
	}

	function selectSupervisorValue(supervisorID: string) {
		record.supervisorID = supervisorID === noSelectionValue ? '' : supervisorID;
	}

	function supervisorOptions() {
		return supervisorCandidatesForRecord(userRecords, record);
	}
</script>

<div class={['mt-3 grid gap-3 border-t pt-3', layout === 'default' ? 'sm:grid-cols-3' : '']}>
	<label class={fieldClass}>
		<Label class="text-xs">{text.organization.jobTitle}</Label>
		<Input class={controlClass} bind:value={record.jobTitle} placeholder={text.organization.jobTitlePlaceholder} autocomplete="off" disabled={isSaving} />
	</label>
	<div class={fieldClass}>
		<Label class="text-xs">{text.organization.organization}</Label>
		<Select.Root type="single" value={record.groupID || noSelectionValue} onValueChange={selectOrganizationValue} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.organization.organization}>
				{organizationLabel()}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={noSelectionValue} label={text.organization.none}>{text.organization.none}</Select.Item>
				{#each groups as group (group.id)}
					<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<div class={fieldClass}>
		<Label class="text-xs">{text.organization.supervisor}</Label>
		<Select.Root type="single" value={record.supervisorID || noSelectionValue} onValueChange={selectSupervisorValue} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.organization.supervisor}>
				{supervisorLabel()}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={noSelectionValue} label={text.organization.none}>{text.organization.none}</Select.Item>
				{#each supervisorOptions() as option (option.userID)}
					<Select.Item value={option.userID} label={personLabel(option)}>{personLabel(option)}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
</div>

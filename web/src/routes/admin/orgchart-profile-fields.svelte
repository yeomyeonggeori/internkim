<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import type { AdminPageText, OrgGroup, UserRecord } from './admin-types';
	import { supervisorCandidatesForRecord } from './orgchart-tree';

	type OrgchartProfileFieldsProps = {
		record: UserRecord;
		userRecords: UserRecord[];
		groups: OrgGroup[];
		text: AdminPageText;
		isSaving: boolean;
	};

	let {
		record = $bindable(),
		userRecords,
		groups,
		text,
		isSaving
	}: OrgchartProfileFieldsProps = $props();

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

	function primaryGroupLabel() {
		return record.primaryGroupID ? groupName(record.primaryGroupID) || text.orgchart.none : text.orgchart.none;
	}

	function supervisorLabel() {
		const supervisor = userRecords.find((candidate) => candidate.userID === record.supervisorID);
		return supervisor ? personLabel(supervisor) : text.orgchart.none;
	}

	function selectPrimaryGroup(groupID: string) {
		record.primaryGroupID = groupID;
		record.group = groupID;
		record.groupIDs = groupID ? [groupID] : [];
	}

	function selectPrimaryGroupValue(groupID: string) {
		selectPrimaryGroup(groupID === noSelectionValue ? '' : groupID);
	}

	function selectSupervisorValue(supervisorID: string) {
		record.supervisorID = supervisorID === noSelectionValue ? '' : supervisorID;
	}

	function supervisorOptions() {
		return supervisorCandidatesForRecord(userRecords, record);
	}
</script>

<div class="mt-3 grid gap-3 border-t pt-3 sm:grid-cols-3">
	<label class={fieldClass}>
		<Label class="text-xs">{text.orgchart.jobTitle}</Label>
		<Input class={controlClass} bind:value={record.jobTitle} placeholder={text.orgchart.jobTitlePlaceholder} autocomplete="off" disabled={isSaving} />
	</label>
	<div class={fieldClass}>
		<Label class="text-xs">{text.orgchart.organization}</Label>
		<Select.Root type="single" value={record.primaryGroupID || noSelectionValue} onValueChange={selectPrimaryGroupValue} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.orgchart.organization}>
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
		<Label class="text-xs">{text.orgchart.supervisor}</Label>
		<Select.Root type="single" value={record.supervisorID || noSelectionValue} onValueChange={selectSupervisorValue} disabled={isSaving}>
			<Select.Trigger class={selectTriggerClass} aria-label={text.orgchart.supervisor}>
				{supervisorLabel()}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={noSelectionValue} label={text.orgchart.none}>{text.orgchart.none}</Select.Item>
				{#each supervisorOptions() as option (option.userID)}
					<Select.Item value={option.userID} label={personLabel(option)}>{personLabel(option)}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
</div>

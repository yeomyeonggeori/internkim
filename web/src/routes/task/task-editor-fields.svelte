<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import TaskPersonChip from './task-person-chip.svelte';
	import {
		taskStatus,
		isTaskStatusInProgress,
		isTaskStatusPlanned,
		isTaskStatusRejected,
		isTaskStatusRequested,
		isTaskStatusStopped
	} from './task-status';
	import { hasTaskRequestProvenance } from './task-options';
	import { taskDefinitionLabel, taskDefinitionOptionValue, taskDefinitionValueFromOption } from './task-workspace-model';
	import type { TaskEditorOption, TaskEditorText } from './task-editor-types';
	import type { Task } from './task-types';

	type Props = {
		taskDraft: Task;
		categoryOptions: TaskEditorOption[];
		typeOptions: TaskEditorOption[];
		sizeOptions: TaskEditorOption[];
		statusOptions: TaskEditorOption[];
		canEditTask: boolean;
		text: TaskEditorText;
		statusLabel: (status: string) => string;
		memberEmail: (memberID: string) => string;
	};

	let {
		taskDraft = $bindable<Task>(),
		categoryOptions,
		typeOptions,
		sizeOptions,
		statusOptions,
		canEditTask,
		text,
		statusLabel,
		memberEmail
	}: Props = $props();

	let businessSelectValue = $derived(taskDefinitionOptionValue(taskDraft.business));
	let typeSelectValue = $derived(taskDefinitionOptionValue(taskDraft.type));

	function updateBusiness(value: string): void {
		taskDraft.business = taskDefinitionValueFromOption(value);
	}

	function updateType(value: string): void {
		taskDraft.type = taskDefinitionValueFromOption(value);
	}

	function completeWhenEndDatePassed(): void {
		if (!isTaskStatusPlanned(taskDraft.status) && !isTaskStatusInProgress(taskDraft.status)) return;
		if (!taskDraft.endDate || taskDraft.endDate > localTodayISO()) return;
		taskDraft.status = taskStatus.completed;
	}

	function localTodayISO(): string {
		const now = new Date();
		return new Date(now.getTime() - now.getTimezoneOffset() * 60_000).toISOString().slice(0, 10);
	}
</script>

<label class="grid gap-1 text-xs font-medium text-muted-foreground">
	{text.content}
	<Input bind:value={taskDraft.content} placeholder={text.contentPlaceholder} disabled={!canEditTask} />
</label>
<div class="grid gap-3 md:grid-cols-2">
	{#if hasTaskRequestProvenance(taskDraft)}
		<div class="grid gap-1 text-xs font-medium text-muted-foreground">
			<span>{text.requester}</span>
			<div class="flex h-8 w-full items-center rounded-lg border border-input bg-transparent px-2.5 text-sm font-normal text-foreground">
				<TaskPersonChip
					name={taskDraft.requesterName || memberEmail(taskDraft.requesterID || '') || taskDraft.requesterID || text.requesterUnavailable}
					email={memberEmail(taskDraft.requesterID || '')}
					seed={taskDraft.requesterID || taskDraft.requesterName || text.requesterUnavailable}
					
				/>
			</div>
		</div>
	{/if}
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.status}
		<Select.Root type="single" bind:value={taskDraft.status} disabled={!canEditTask}>
			<Select.Trigger class="w-full">
				{statusLabel(taskDraft.status)}
			</Select.Trigger>
			<Select.Content>
				{#each statusOptions as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
	{#if categoryOptions.length > 1}
		<label class="grid gap-1 text-xs font-medium text-muted-foreground">
			{text.business}
			<Select.Root type="single" value={businessSelectValue} onValueChange={updateBusiness} disabled={!canEditTask}>
				<Select.Trigger class="w-full">
					{taskDefinitionLabel(taskDraft.business, text.etcLabel)}
				</Select.Trigger>
				<Select.Content>
					{#each categoryOptions as option (option.value)}
						<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</label>
	{/if}
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.type}
		<Select.Root type="single" value={typeSelectValue} onValueChange={updateType} disabled={!canEditTask}>
			<Select.Trigger class="w-full">
				{taskDefinitionLabel(taskDraft.type, text.etcLabel)}
			</Select.Trigger>
			<Select.Content>
				{#each typeOptions as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.size}
		<Select.Root type="single" bind:value={taskDraft.size} disabled={!canEditTask}>
			<Select.Trigger class="w-full">
				{sizeOptions.find((option) => option.value === taskDraft.size)?.label ?? '-'}
			</Select.Trigger>
			<Select.Content>
				{#each sizeOptions as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.startDate}
		<Input type="date" bind:value={taskDraft.startDate} disabled={!canEditTask} />
	</label>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.endDate}
		<Input type="date" bind:value={taskDraft.endDate} disabled={!canEditTask} onchange={completeWhenEndDatePassed} />
	</label>
</div>

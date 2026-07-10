<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { isFlowStatusRejected, isFlowStatusRequested, isFlowStatusStopped } from './flow-status';
	import { flowBusinessLabel, flowBusinessOptionValue, flowBusinessValueFromOption } from './flow-task-workspace-model';
	import type { FlowTaskEditorOption, FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowMember, FlowTask } from './flow-types';

	type Props = {
		taskDraft: FlowTask;
		members: FlowMember[];
		memberOptions: FlowTaskEditorOption[];
		categoryOptions: FlowTaskEditorOption[];
		typeOptions: FlowTaskEditorOption[];
		sizeOptions: FlowTaskEditorOption[];
		statusOptions: FlowTaskEditorOption[];
		canEditTask: boolean;
		canEditTaskAssignment: boolean;
		text: FlowTaskEditorText;
		statusLabel: (status: string) => string;
		setTaskOwnerID: (memberID: string) => void;
	};

	let {
		taskDraft = $bindable<FlowTask>(),
		members,
		memberOptions,
		categoryOptions,
		typeOptions,
		sizeOptions,
		statusOptions,
		canEditTask,
		canEditTaskAssignment,
		text,
		statusLabel,
		setTaskOwnerID
	}: Props = $props();

	let businessSelectValue = $derived(flowBusinessOptionValue(taskDraft.business));

	function memberOptionLabel(memberID: string): string {
		return memberOptions.find((option) => option.value === memberID)?.label ?? '-';
	}

	function memberOptionEmail(memberID: string): string {
		return members.find((member) => member.id === memberID)?.email ?? '';
	}

	function memberOptionImage(memberID: string): string {
		return members.find((member) => member.id === memberID)?.image ?? '';
	}

	function updateBusiness(value: string): void {
		taskDraft.business = flowBusinessValueFromOption(value);
	}
</script>

<label class="grid gap-1 text-xs font-medium text-muted-foreground">
	{text.content}
	<Input bind:value={taskDraft.content} placeholder={text.contentPlaceholder} disabled={!canEditTask} />
</label>
<label class="grid gap-1 text-xs font-medium text-muted-foreground">
	{text.goal}
	<Input bind:value={taskDraft.goal} placeholder={text.goalPlaceholder} disabled={!canEditTask} />
</label>
<div class="grid gap-3 md:grid-cols-2">
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.owner}
		<Select.Root type="single" value={taskDraft.ownerID} onValueChange={setTaskOwnerID} disabled={!canEditTask || !canEditTaskAssignment}>
			<Select.Trigger class="w-full">
				<span class="flex min-w-0 items-center gap-2">
					<PersonAvatar name={memberOptionLabel(taskDraft.ownerID)} email={memberOptionEmail(taskDraft.ownerID)} seed={taskDraft.ownerID} image={memberOptionImage(taskDraft.ownerID)} class="size-5" />
					<span class="truncate">{memberOptionLabel(taskDraft.ownerID)}</span>
				</span>
			</Select.Trigger>
			<Select.Content>
				{#each memberOptions as option (option.value)}
					<Select.Item value={option.value} label={option.label}>
						<span class="flex min-w-0 items-center gap-2">
							<PersonAvatar name={option.label} email={memberOptionEmail(option.value)} seed={option.value} image={memberOptionImage(option.value)} class="size-5" />
							<span class="truncate">{option.label}</span>
						</span>
					</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
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
					{flowBusinessLabel(taskDraft.business, text.businessFallback)}
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
		<Select.Root type="single" bind:value={taskDraft.type} disabled={!canEditTask}>
			<Select.Trigger class="w-full">
				{taskDraft.type || '-'}
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
		<Input type="date" bind:value={taskDraft.endDate} disabled={!canEditTask} />
	</label>
</div>
{#if isFlowStatusRequested(taskDraft.status)}
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.requestReason}
		<Input bind:value={taskDraft.requestReason} disabled={!canEditTask} />
	</label>
{/if}
{#if isFlowStatusRejected(taskDraft.status) || isFlowStatusStopped(taskDraft.status)}
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{text.reason}
		<Input bind:value={taskDraft.decisionReason} disabled={!canEditTask} />
	</label>
{/if}

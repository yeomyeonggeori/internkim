<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { isFlowStatusRejected, isFlowStatusRequested, isFlowStatusStopped } from './flow-status';
	import { flowBusinessLabel, flowBusinessOptionValue, flowBusinessValueFromOption } from './flow-task-workspace-model';
	import type { FlowTaskEditorOption, FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowTask } from './flow-types';

	type Props = {
		taskDraft: FlowTask;
		categoryOptions: FlowTaskEditorOption[];
		typeOptions: FlowTaskEditorOption[];
		sizeOptions: FlowTaskEditorOption[];
		statusOptions: FlowTaskEditorOption[];
		canEditTask: boolean;
		text: FlowTaskEditorText;
		statusLabel: (status: string) => string;
	};

	let {
		taskDraft = $bindable<FlowTask>(),
		categoryOptions,
		typeOptions,
		sizeOptions,
		statusOptions,
		canEditTask,
		text,
		statusLabel
	}: Props = $props();

	let businessSelectValue = $derived(flowBusinessOptionValue(taskDraft.business));

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
{#if taskDraft.requesterID}
	<div class="grid gap-1 text-xs font-medium text-muted-foreground">
		<span>{text.requester}</span>
		<span class="text-sm font-normal text-foreground">{taskDraft.requesterName || taskDraft.requesterID}</span>
	</div>
{/if}
<div class="grid gap-3 md:grid-cols-2">
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

<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import { statusBadgeClass } from './flow-style';
	import type { FlowTask } from './flow-types';

	type Option = {
		value: string;
		label: string;
	};

	type Props = {
		task: FlowTask;
		statusOptions: Option[];
		pendingStatusTaskID: string;
		statusLabel: (status: string) => string;
		updateTaskStatus: (task: FlowTask, nextStatus: string) => Promise<void>;
		canUpdateTask: (task: FlowTask) => boolean;
	};

	let {
		task,
		statusOptions,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		canUpdateTask
	}: Props = $props();
</script>

<div onclick={(event) => event.stopPropagation()} role="presentation">
	<Select.Root
		type="single"
		value={task.status}
		disabled={pendingStatusTaskID === task.id || !canUpdateTask(task)}
		onValueChange={(next) => updateTaskStatus(task, next)}
	>
		<Select.Trigger
			size="sm"
			class={cn('w-28 justify-between border-transparent font-medium', statusBadgeClass(task.status))}
		>
			{statusLabel(task.status)}
		</Select.Trigger>
		<Select.Content>
			{#each statusOptions as option (option.value)}
				<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
</div>

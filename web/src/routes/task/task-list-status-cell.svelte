<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import { statusBadgeClass } from './task-style';
	import type { Task } from './task-types';

	type Option = {
		value: string;
		label: string;
	};

	type Props = {
		task: Task;
		statusOptionsForTask: (task: Task) => Option[];
		pendingStatusTaskID: string;
		statusLabel: (status: string) => string;
		updateTaskStatus: (task: Task, nextStatus: string) => Promise<void>;
		canUpdateTask: (task: Task) => boolean;
	};

	let {
		task,
		statusOptionsForTask,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		canUpdateTask
	}: Props = $props();

	let statusOptions = $derived(statusOptionsForTask(task));
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
		<Select.Content><Select.Group>
			{#each statusOptions as option (option.value)}
				<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
			{/each}
		</Select.Group></Select.Content>
	</Select.Root>
</div>

<script lang="ts">
	import { cn } from '$lib/utils';
	import { taskDateRangeParts, taskDateText } from './task-date-range';
	import type { Task } from './task-types';

	type Props = {
		task: Task;
		businessColor: string;
		isOverduePlan?: boolean;
		class?: string;
	};

	let { task, businessColor, isOverduePlan = false, class: className }: Props = $props();

	const dateRangeText = $derived(
		taskDateRangeParts(task.startDate, task.endDate, new Date().getFullYear()).map(taskDateText).join(' – ')
	);
</script>

<div class={cn('flex min-w-0 flex-wrap items-center gap-x-1.5 text-xs leading-5 text-muted-foreground', className)} data-task-compact-metadata>
	{#if task.business}
		<span class="inline-flex min-w-0 items-center gap-1.5">
			<span class="size-2 shrink-0 rounded-[2px]" style:background-color={businessColor} aria-hidden="true"></span>
			<span class="truncate">{task.business}</span>
		</span>
	{/if}
	{#if task.type}
		{#if task.business}<span aria-hidden="true" class="opacity-50">·</span>{/if}
		<span class="truncate">{task.type}</span>
	{/if}
	{#if dateRangeText}
		{#if task.business || task.type}<span aria-hidden="true" class="opacity-50">·</span>{/if}
		<span class={cn('tabular-nums', isOverduePlan && 'font-medium text-destructive')}>{dateRangeText}</span>
	{/if}
</div>

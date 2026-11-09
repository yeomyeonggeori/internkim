<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import {
		formatTaskRoadmapMonth,
		shiftTaskRoadmapMonth,
		taskRoadmapMonthFromKey,
		taskRoadmapMonthKey,
		type TaskRoadmapMonth
	} from './task-roadmap-model';

	type Props = {
		month: TaskRoadmapMonth;
		currentMonth: TaskRoadmapMonth;
		locale: string;
		selectMonthLabel: string;
		currentMonthLabel: string;
		previousMonthLabel: string;
		nextMonthLabel: string;
		onSelectMonth: (month: TaskRoadmapMonth) => void;
		class?: string;
	};

	let {
		month,
		currentMonth,
		locale,
		selectMonthLabel,
		currentMonthLabel,
		previousMonthLabel,
		nextMonthLabel,
		onSelectMonth,
		class: className
	}: Props = $props();

	const monthChoiceRadius = 6;
	const monthChoices = $derived(
		Array.from({ length: monthChoiceRadius * 2 + 1 }, (_, index) => {
			const offset = index - monthChoiceRadius;
			const choice = shiftTaskRoadmapMonth(currentMonth, offset);
			return { value: taskRoadmapMonthKey(choice), label: offset === 0 ? currentMonthLabel : formatTaskRoadmapMonth(choice, locale) };
		})
	);
	let selectedMonthKey = $state('');

	$effect(() => {
		selectedMonthKey = taskRoadmapMonthKey(month);
	});

	function selectMonthKey(key: string): void {
		const selected = taskRoadmapMonthFromKey(key);
		if (selected) onSelectMonth(selected);
	}
</script>

<div class={cn('flex shrink-0 items-center gap-1', className)}>
	<TooltipIconButton label={previousMonthLabel} variant="outline" size="icon-sm" class="max-sm:hidden" onclick={() => onSelectMonth(shiftTaskRoadmapMonth(month, -1))}>
		<ChevronLeftIcon />
	</TooltipIconButton>
	<Select.Root type="single" bind:value={selectedMonthKey} onValueChange={selectMonthKey}>
		<Select.Trigger class="h-8 w-auto justify-between sm:w-[9.5rem] rounded-[min(var(--radius-md),10px)] tabular-nums" aria-label={selectMonthLabel}>
			{formatTaskRoadmapMonth(month, locale)}
		</Select.Trigger>
		<Select.Content><Select.Group>
			{#each monthChoices as choice (choice.value)}
				<Select.Item value={choice.value} label={choice.label}>{choice.label}</Select.Item>
			{/each}
		</Select.Group></Select.Content>
	</Select.Root>
	<TooltipIconButton label={nextMonthLabel} variant="outline" size="icon-sm" class="max-sm:hidden" onclick={() => onSelectMonth(shiftTaskRoadmapMonth(month, 1))}>
		<ChevronRightIcon />
	</TooltipIconButton>
</div>

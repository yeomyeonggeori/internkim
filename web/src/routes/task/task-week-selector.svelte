<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { formatTaskWeekDateRange, taskWeekOptions } from './task-week-label';
	import type { TaskWeek } from './task-types';

	type Props = {
		week: TaskWeek | null | undefined;
		currentWeekStartISO: string;
		disabled: boolean;
		selectWeekLabel: string;
		currentWeekLabel: string;
		lastWeekLabel: string;
		previousWeekLabel: string;
		nextWeekLabel: string;
		onSelectWeek: (weekCode: string) => void;
		class?: string;
	};

	let {
		week,
		currentWeekStartISO,
		disabled,
		selectWeekLabel,
		currentWeekLabel,
		lastWeekLabel,
		previousWeekLabel,
		nextWeekLabel,
		onSelectWeek,
		class: className
	}: Props = $props();

	const weekChoices = $derived(
		taskWeekOptions(currentWeekStartISO || week?.startISO || '', 12, 4).map((option) => ({
			value: option.value,
			label: weekChoiceLabel(option.offsetFromCurrent, option.label)
		}))
	);
	const selectedWeekLabel = $derived(week?.startISO ? formatTaskWeekDateRange(week) : selectWeekLabel);
	let selectedWeekCode = $state('');

	$effect(() => {
		selectedWeekCode = week?.code ?? '';
	});

	function weekChoiceLabel(offsetFromCurrent: number, dateRangeLabel: string): string {
		if (offsetFromCurrent === 0) return currentWeekLabel;
		if (offsetFromCurrent === -1) return lastWeekLabel;
		if (offsetFromCurrent === 1) return nextWeekLabel;
		return dateRangeLabel;
	}
</script>

<div class={cn('flex shrink-0 items-center gap-1', className)}>
	<TooltipIconButton label={previousWeekLabel} variant="outline" size="icon-sm" {disabled} onclick={() => onSelectWeek(week?.previous ?? '')}>
		<ChevronLeftIcon />
	</TooltipIconButton>
	<Select.Root type="single" bind:value={selectedWeekCode} {disabled} onValueChange={(weekCode) => weekCode && onSelectWeek(weekCode)}>
		<Select.Trigger class="h-8 w-[9.5rem] justify-between rounded-[min(var(--radius-md),10px)] tabular-nums" aria-label={selectWeekLabel}>
			{selectedWeekLabel}
		</Select.Trigger>
		<Select.Content><Select.Group>
			{#each weekChoices as choice (choice.value)}
				<Select.Item value={choice.value} label={choice.label}>{choice.label}</Select.Item>
			{/each}
		</Select.Group></Select.Content>
	</Select.Root>
	<TooltipIconButton label={nextWeekLabel} variant="outline" size="icon-sm" {disabled} onclick={() => onSelectWeek(week?.next ?? '')}>
		<ChevronRightIcon />
	</TooltipIconButton>
</div>

<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { flowWeekOptions } from './flow-week-label';
	import type { FlowWeek } from './flow-types';

	type Props = {
		week: FlowWeek | null | undefined;
		currentWeekStartISO: string;
		disabled: boolean;
		selectWeekLabel: string;
		currentWeekLabel: string;
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
		previousWeekLabel,
		nextWeekLabel,
		onSelectWeek,
		class: className
	}: Props = $props();

	const weekChoices = $derived(
		flowWeekOptions(currentWeekStartISO || week?.startISO || '', 12, 4).map((option) => ({
			value: option.value,
			label: weekChoiceLabel(option.offsetFromCurrent, option.label)
		}))
	);
	const selectedWeekLabel = $derived(weekChoices.find((choice) => choice.value === week?.code)?.label ?? selectWeekLabel);
	let selectedWeekCode = $state('');

	$effect(() => {
		selectedWeekCode = week?.code ?? '';
	});

	function weekChoiceLabel(offsetFromCurrent: number, dateRangeLabel: string): string {
		if (offsetFromCurrent === 0) return currentWeekLabel;
		if (offsetFromCurrent === -1) return previousWeekLabel;
		if (offsetFromCurrent === 1) return nextWeekLabel;
		return dateRangeLabel;
	}
</script>

<div class={cn('flex shrink-0 items-center gap-1', className)}>
	<TooltipIconButton label={previousWeekLabel} variant="outline" size="icon-sm" {disabled} onclick={() => onSelectWeek(week?.previous ?? '')}>
		<ChevronLeftIcon />
	</TooltipIconButton>
	<Select.Root type="single" bind:value={selectedWeekCode} {disabled} onValueChange={(weekCode) => weekCode && onSelectWeek(weekCode)}>
		<Select.Trigger size="sm" class="w-[9.5rem] justify-between tabular-nums" aria-label={selectWeekLabel}>
			{selectedWeekLabel}
		</Select.Trigger>
		<Select.Content>
			{#each weekChoices as choice (choice.value)}
				<Select.Item value={choice.value} label={choice.label}>{choice.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
	<TooltipIconButton label={nextWeekLabel} variant="outline" size="icon-sm" {disabled} onclick={() => onSelectWeek(week?.next ?? '')}>
		<ChevronRightIcon />
	</TooltipIconButton>
</div>

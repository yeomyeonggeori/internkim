<script lang="ts">
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { cn } from '$lib/utils';
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
			label: option.isCurrent ? `${option.label} · ${currentWeekLabel}` : option.label
		}))
	);
	let selectedWeekCode = $state('');

	$effect(() => {
		selectedWeekCode = week?.code ?? '';
	});
</script>

<div class={cn('flex shrink-0 items-center gap-1', className)}>
	<TooltipIconButton label={previousWeekLabel} variant="outline" size="icon-sm" {disabled} onclick={() => onSelectWeek(week?.previous ?? '')}>
		<ChevronLeftIcon />
	</TooltipIconButton>
	<FilterCombobox
		bind:value={selectedWeekCode}
		options={weekChoices}
		label={selectWeekLabel}
		clearValue=""
		onSelect={(weekCode) => weekCode && onSelectWeek(weekCode)}
		class="h-8 w-[11rem] tabular-nums"
	/>
	<TooltipIconButton label={nextWeekLabel} variant="outline" size="icon-sm" {disabled} onclick={() => onSelectWeek(week?.next ?? '')}>
		<ChevronRightIcon />
	</TooltipIconButton>
</div>

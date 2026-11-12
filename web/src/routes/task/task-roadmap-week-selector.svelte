<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { cn } from '$lib/utils';
	import { formatTaskRoadmapWindow, taskRoadmapDayCount, taskRoadmapWeekChoices, taskRoadmapWeekStart } from './task-roadmap-window';

	type Props = {
		startISO: string;
		todayISO: string;
		selectWeekLabel: string;
		currentWeekLabel: string;
		onSelectWeek: (weekStartISO: string) => void;
		class?: string;
	};

	let { startISO, todayISO, selectWeekLabel, currentWeekLabel, onSelectWeek, class: className }: Props = $props();

	const isNarrow = new IsMobile();
	const dayCount = $derived(taskRoadmapDayCount(isNarrow.current));
	const currentWeekStart = $derived(taskRoadmapWeekStart(todayISO));
	const weekChoices = $derived(
		taskRoadmapWeekChoices(todayISO).map((weekStartISO) => ({
			value: weekStartISO,
			label: weekStartISO === currentWeekStart ? currentWeekLabel : formatTaskRoadmapWindow(weekStartISO, dayCount, todayISO)
		}))
	);
	let selectedWeekStart = $state('');

	$effect(() => {
		selectedWeekStart = taskRoadmapWeekStart(startISO) === startISO ? startISO : '';
	});
</script>

<Select.Root type="single" bind:value={selectedWeekStart} onValueChange={(weekStartISO) => weekStartISO && onSelectWeek(weekStartISO)}>
	<Select.Trigger class={cn('h-8 w-auto shrink-0 justify-between sm:w-[11rem] rounded-[min(var(--radius-md),10px)] tabular-nums', className)} aria-label={selectWeekLabel}>
		{formatTaskRoadmapWindow(startISO, dayCount, todayISO)}
	</Select.Trigger>
	<Select.Content><Select.Group>
		{#each weekChoices as choice (choice.value)}
			<Select.Item value={choice.value} label={choice.label}>{choice.label}</Select.Item>
		{/each}
	</Select.Group></Select.Content>
</Select.Root>

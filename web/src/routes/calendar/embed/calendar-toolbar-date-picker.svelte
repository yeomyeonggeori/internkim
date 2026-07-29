<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { Button } from '$lib/components/ui/button';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { CalendarLocaleText } from '../text';

	type CalendarToolbarDatePickerText = Pick<
		CalendarLocaleText,
		'pickMonthAndYear' | 'previousYear' | 'nextYear' | 'previousTwelveYears' | 'nextTwelveYears'
	>;

	type CalendarToolbarDatePickerProps = {
		selectedDate: Date;
		localeCode: string;
		text: CalendarToolbarDatePickerText;
		selectDate: (date: Date) => void;
	};

	type PickerMode = 'month' | 'year';

	const yearsPerPage = 12;

	let { selectedDate, localeCode, text, selectDate }: CalendarToolbarDatePickerProps = $props();

	let pickerMode = $state<PickerMode>('month');
	let pickedYear = $state<number | null>(null);
	let pickedYearWindowStart = $state<number | null>(null);

	const pickerYear = $derived(pickedYear ?? selectedDate.getFullYear());
	const yearWindowStart = $derived(pickedYearWindowStart ?? startOfYearWindow(pickerYear));

	const monthLabels = $derived(
		Array.from({ length: 12 }, (_, monthIndex) =>
			new Date(2000, monthIndex, 1).toLocaleDateString(localeCode, { month: 'short' })
		)
	);
	const yearOptions = $derived(Array.from({ length: yearsPerPage }, (_, index) => yearWindowStart + index));
	const pageLabel = $derived(
		pickerMode === 'month' ? String(pickerYear) : `${yearWindowStart} – ${yearWindowStart + yearsPerPage - 1}`
	);
	const previousLabel = $derived(pickerMode === 'month' ? text.previousYear : text.previousTwelveYears);
	const nextLabel = $derived(pickerMode === 'month' ? text.nextYear : text.nextTwelveYears);

	function startOfYearWindow(year: number): number {
		return Math.floor(year / yearsPerPage) * yearsPerPage;
	}

	function daysInMonth(year: number, monthIndex: number): number {
		return new Date(year, monthIndex + 1, 0).getDate();
	}

	function shiftPickerPage(direction: -1 | 1): void {
		if (pickerMode === 'month') {
			pickedYear = pickerYear + direction;
			pickedYearWindowStart = startOfYearWindow(pickerYear + direction);
			return;
		}
		pickedYearWindowStart = yearWindowStart + direction * yearsPerPage;
	}

	function togglePickerMode(): void {
		pickerMode = pickerMode === 'month' ? 'year' : 'month';
		pickedYearWindowStart = startOfYearWindow(pickerYear);
	}

	function selectMonth(monthIndex: number): void {
		const selectedDay = Math.min(selectedDate.getDate(), daysInMonth(pickerYear, monthIndex));
		selectDate(new Date(pickerYear, monthIndex, selectedDay, 12, 0, 0, 0));
	}

	function selectYear(year: number): void {
		pickedYear = year;
		pickerMode = 'month';
	}

	function isSelectedMonth(monthIndex: number): boolean {
		return pickerYear === selectedDate.getFullYear() && monthIndex === selectedDate.getMonth();
	}
</script>

<div class="grid gap-3" aria-label={text.pickMonthAndYear}>
	<div class="flex items-center justify-between gap-1">
		<TooltipIconButton label={previousLabel} variant="ghost" size="icon-sm" onclick={() => shiftPickerPage(-1)}>
			<ChevronLeftIcon />
		</TooltipIconButton>
		<Button variant="ghost" size="sm" class="font-semibold tabular-nums" onclick={togglePickerMode}>
			{pageLabel}
		</Button>
		<TooltipIconButton label={nextLabel} variant="ghost" size="icon-sm" onclick={() => shiftPickerPage(1)}>
			<ChevronRightIcon />
		</TooltipIconButton>
	</div>
	{#if pickerMode === 'month'}
		<div class="grid grid-cols-3 gap-1">
			{#each monthLabels as monthLabel, monthIndex (monthLabel)}
				<Button variant={isSelectedMonth(monthIndex) ? 'default' : 'ghost'} size="sm" onclick={() => selectMonth(monthIndex)}>
					{monthLabel}
				</Button>
			{/each}
		</div>
	{:else}
		<div class="grid grid-cols-3 gap-1">
			{#each yearOptions as year (year)}
				<Button
					variant={year === selectedDate.getFullYear() ? 'default' : 'ghost'}
					size="sm"
					class="tabular-nums"
					onclick={() => selectYear(year)}
				>
					{year}
				</Button>
			{/each}
		</div>
	{/if}
</div>

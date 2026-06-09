<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { Popover } from 'bits-ui';
	import { calendarText } from './text';

	type MiniMonthCell = { date: Date; isOther: boolean; isToday: boolean };

	type Props = {
		month: Date;
		eventDates: Set<string>;
		selectedDateKey: string;
		onMonthChange: (month: Date) => void;
		onSelectDate: (date: Date) => void;
	};

	let { month, eventDates, selectedDateKey, onMonthChange, onSelectDate }: Props = $props();

	const text = createPageText(calendarText);
	const today = new Date();
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const monthLabel = $derived(month.toLocaleDateString(localeCode, { year: 'numeric', month: 'long' }));
	const monthShortLabels = $derived(
		Array.from({ length: 12 }, (_, monthIndex) => new Date(2000, monthIndex, 1).toLocaleDateString(localeCode, { month: 'short' }))
	);
	const weekdayLabels = $derived(
		Array.from({ length: 7 }, (_, weekdayIndex) => new Date(2026, 4, 24 + weekdayIndex).toLocaleDateString(localeCode, { weekday: 'narrow' }))
	);

	let isMonthPickerOpen = $state(false);
	let pickerMode = $state<'month' | 'year'>('month');
	let pickerYear = $state(today.getFullYear());
	let pickerYearWindowStart = $state(Math.floor(today.getFullYear() / 12) * 12);

	function handlePickerOpenChange(open: boolean) {
		isMonthPickerOpen = open;
		if (open) {
			pickerMode = 'month';
			pickerYear = month.getFullYear();
			pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
		}
	}

	function shiftMonth(delta: number) {
		onMonthChange(new Date(month.getFullYear(), month.getMonth() + delta, 1));
	}

	function selectPickerMonth(monthIndex: number) {
		onMonthChange(new Date(pickerYear, monthIndex, 1));
		isMonthPickerOpen = false;
	}

	function selectPickerYear(year: number) {
		pickerYear = year;
		pickerMode = 'month';
	}

	function shiftPickerRange(delta: number) {
		if (pickerMode === 'month') pickerYear += delta;
		else pickerYearWindowStart += delta * 12;
	}

	function togglePickerMode() {
		if (pickerMode === 'month') {
			pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
			pickerMode = 'year';
			return;
		}
		pickerMode = 'month';
	}

	function isSameDay(left: Date, right: Date) {
		return left.getFullYear() === right.getFullYear() && left.getMonth() === right.getMonth() && left.getDate() === right.getDate();
	}

	function dateKey(date: Date): string {
		return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
	}

	function miniMonthCells(): MiniMonthCell[] {
		const year = month.getFullYear();
		const monthIndex = month.getMonth();
		const firstWeekday = new Date(year, monthIndex, 1).getDay();
		const cells: MiniMonthCell[] = [];
		for (let dayOffset = firstWeekday; dayOffset > 0; dayOffset -= 1) {
			const date = new Date(year, monthIndex, 1 - dayOffset);
			cells.push({ date, isOther: true, isToday: isSameDay(date, today) });
		}
		const daysInMonth = new Date(year, monthIndex + 1, 0).getDate();
		for (let day = 1; day <= daysInMonth; day += 1) {
			const date = new Date(year, monthIndex, day);
			cells.push({ date, isOther: false, isToday: isSameDay(date, today) });
		}
		let trailingDay = 1;
		while (cells.length < 42) {
			const date = new Date(year, monthIndex + 1, trailingDay);
			cells.push({ date, isOther: true, isToday: isSameDay(date, today) });
			trailingDay += 1;
		}
		return cells;
	}
</script>

<section class="shrink-0 border-t px-5 pb-3 pt-4">
	<header class="mb-2 flex items-center gap-1">
		<button
			type="button"
			aria-label={text.previousMonth}
			class="flex size-7 items-center justify-center rounded text-foreground hover:bg-accent"
			onclick={() => shiftMonth(-1)}
		>
			<ChevronLeftIcon class="size-3.5" />
		</button>
		<Popover.Root open={isMonthPickerOpen} onOpenChange={handlePickerOpenChange}>
			<Popover.Trigger
				class="flex h-7 flex-1 items-center justify-center rounded text-center text-[14px] font-bold transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
				aria-label={text.pickMonthAndYear}
			>
				{monthLabel}
			</Popover.Trigger>
			<Popover.Portal>
				<Popover.Content
					side="bottom"
					align="center"
					sideOffset={6}
					class="z-50 w-56 rounded-md border border-border/50 bg-popover p-2 text-popover-foreground shadow-sm outline-none data-open:animate-in data-closed:animate-out data-open:fade-in-0 data-closed:fade-out-0 data-open:zoom-in-95 data-closed:zoom-out-95"
				>
					<header class="mb-1.5 flex items-center gap-1">
						<button
							type="button"
							aria-label={pickerMode === 'month' ? text.previousYear : text.previousTwelveYears}
							class="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
							onclick={() => shiftPickerRange(-1)}
						>
							<ChevronLeftIcon class="size-3.5" />
						</button>
						<button type="button" class="flex-1 rounded py-1 text-xs font-semibold tabular-nums hover:bg-accent" onclick={togglePickerMode}>
							{#if pickerMode === 'month'}
								{pickerYear}
							{:else}
								{pickerYearWindowStart}-{pickerYearWindowStart + 11}
							{/if}
						</button>
						<button
							type="button"
							aria-label={pickerMode === 'month' ? text.nextYear : text.nextTwelveYears}
							class="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
							onclick={() => shiftPickerRange(1)}
						>
							<ChevronRightIcon class="size-3.5" />
						</button>
					</header>

					{#if pickerMode === 'month'}
						<div class="grid grid-cols-3 gap-1">
							{#each monthShortLabels as label, monthIndex (monthIndex)}
								<button
									type="button"
									class="flex h-8 items-center justify-center rounded text-xs tabular-nums transition-colors {pickerYear === month.getFullYear() && monthIndex === month.getMonth()
										? 'bg-primary font-semibold text-primary-foreground'
										: pickerYear === today.getFullYear() && monthIndex === today.getMonth()
											? 'font-semibold text-primary hover:bg-accent'
											: 'text-foreground hover:bg-accent'}"
									onclick={() => selectPickerMonth(monthIndex)}
								>
									{label}
								</button>
							{/each}
						</div>
					{:else}
						<div class="grid grid-cols-3 gap-1">
							{#each Array.from({ length: 12 }, (_, index) => pickerYearWindowStart + index) as year (year)}
								<button
									type="button"
									class="flex h-8 items-center justify-center rounded text-xs tabular-nums transition-colors {year === pickerYear
										? 'bg-primary font-semibold text-primary-foreground'
										: year === today.getFullYear()
											? 'font-semibold text-primary hover:bg-accent'
											: 'text-foreground hover:bg-accent'}"
									onclick={() => selectPickerYear(year)}
								>
									{year}
								</button>
							{/each}
						</div>
					{/if}
				</Popover.Content>
			</Popover.Portal>
		</Popover.Root>
		<button
			type="button"
			aria-label={text.nextMonth}
			class="flex size-7 items-center justify-center rounded text-foreground hover:bg-accent"
			onclick={() => shiftMonth(1)}
		>
			<ChevronRightIcon class="size-3.5" />
		</button>
	</header>
	<div class="grid grid-cols-7 gap-y-0.5">
		{#each weekdayLabels as weekdayLabel, index (index)}
			<span class="py-0.5 text-center text-[11px] font-bold text-muted-foreground">
				{weekdayLabel}
			</span>
		{/each}
		{#each miniMonthCells() as cell, index (index)}
			{@const isWeekend = cell.date.getDay() === 0 || cell.date.getDay() === 6}
			{@const cellDateKey = dateKey(cell.date)}
			{@const hasEvent = eventDates.has(cellDateKey)}
			{@const isSelected = selectedDateKey === cellDateKey}
			<button
				type="button"
				aria-pressed={isSelected}
				data-mini-date-key={cellDateKey}
				class="relative mx-auto flex size-7 items-start justify-center rounded-md pt-0.5 text-[12px] tabular-nums transition-colors {isSelected
					? 'bg-primary font-bold text-primary-foreground'
					: cell.isToday
						? 'font-bold text-primary ring-1 ring-primary/60'
						: cell.isOther
							? 'text-muted-foreground opacity-45 hover:bg-accent'
							: isWeekend
								? 'text-foreground hover:bg-accent'
								: 'text-foreground hover:bg-accent'}"
				onclick={() => onSelectDate(cell.date)}
			>
				{cell.date.getDate()}
				{#if hasEvent}
					<span
						class="absolute bottom-0.5 left-1/2 size-1.5 -translate-x-1/2 rounded-full {isSelected ? 'bg-primary-foreground' : 'bg-primary'}"
						aria-hidden="true"
					></span>
				{/if}
			</button>
		{/each}
	</div>
</section>

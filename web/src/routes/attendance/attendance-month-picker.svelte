<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { Popover } from 'bits-ui';
	import { attendanceText } from './text';

	type Props = {
		selectedMonth: string;
		onSelectMonth: (month: string) => void;
		density?: 'default' | 'compact';
	};

	let { selectedMonth, onSelectMonth, density = 'default' }: Props = $props();

	const text = createPageText(attendanceText);
	const today = new Date();
	let isOpen = $state(false);
	let pickerMode = $state<'month' | 'year'>('month');
	let pickerYear = $state(today.getFullYear());
	let pickerYearWindowStart = $state(Math.floor(today.getFullYear() / 12) * 12);

	const selectedYearMonth = $derived(parseYearMonth(selectedMonth));
	const rootSpacingClass = $derived(density === 'compact' ? 'gap-0' : 'gap-1');
	const navigationButtonClass = $derived(density === 'compact' ? 'size-7' : 'size-8');
	const triggerWidthClass = $derived(
		currentLocale.value === 'en'
			? density === 'compact' ? 'w-[8.5rem]' : 'w-[9.25rem]'
			: density === 'compact' ? 'w-[6.75rem]' : 'w-[7.25rem]'
	);
	const previousPickerLabel = $derived(pickerMode === 'month' ? text.previousYear : text.previousYearWindow);
	const nextPickerLabel = $derived(pickerMode === 'month' ? text.nextYear : text.nextYearWindow);

	function parseYearMonth(value: string): { year: number; month: number } {
		const [yearString, monthString] = value.split('-');
		const year = Number(yearString);
		const month = Number(monthString);
		if (!year || !month) return { year: today.getFullYear(), month: today.getMonth() + 1 };
		return { year, month };
	}

	function formatYearMonth(year: number, monthIndex: number): string {
		return `${year}-${String(monthIndex + 1).padStart(2, '0')}`;
	}

	const triggerLabel = $derived(
		currentLocale.value === 'en'
			? new Date(selectedYearMonth.year, selectedYearMonth.month - 1, 1).toLocaleDateString(text.dateLocale, {
					year: 'numeric',
					month: 'long',
				})
			: `${selectedYearMonth.year}년 ${selectedYearMonth.month}월`
	);

	const monthShortLabels = $derived(
		Array.from({ length: 12 }, (_, index) =>
			currentLocale.value === 'en'
				? new Date(2000, index, 1).toLocaleDateString('en-US', { month: 'short' })
				: `${index + 1}월`
		)
	);

	function handleOpenChange(open: boolean) {
		isOpen = open;
		if (open) {
			pickerMode = 'month';
			pickerYear = selectedYearMonth.year;
			pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
		}
	}

	function selectMonth(monthIndex: number) {
		onSelectMonth(formatYearMonth(pickerYear, monthIndex));
		isOpen = false;
	}

	function moveSelectedMonth(offset: number) {
		const date = new Date(Date.UTC(selectedYearMonth.year, selectedYearMonth.month - 1 + offset, 1));
		isOpen = false;
		onSelectMonth(formatYearMonth(date.getUTCFullYear(), date.getUTCMonth()));
	}

	function selectYear(year: number) {
		pickerYear = year;
		pickerMode = 'month';
	}

	function movePickerBackward() {
		if (pickerMode === 'month') pickerYear -= 1;
		else pickerYearWindowStart -= 12;
	}

	function movePickerForward() {
		if (pickerMode === 'month') pickerYear += 1;
		else pickerYearWindowStart += 12;
	}

	function togglePickerMode() {
		if (pickerMode === 'month') {
			pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
			pickerMode = 'year';
		} else {
			pickerMode = 'month';
		}
	}
</script>

<div class={`inline-flex items-center ${rootSpacingClass}`}>
	<button
		type="button"
		aria-label={text.previousMonth}
		class={`flex ${navigationButtonClass} items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring`}
		onclick={() => moveSelectedMonth(-1)}
	>
		<ChevronLeftIcon class="size-4" />
	</button>
	<Popover.Root open={isOpen} onOpenChange={handleOpenChange}>
		<Popover.Trigger
			class={`border-input bg-background hover:bg-accent flex h-8 items-center justify-center rounded-md border px-3 text-sm font-medium tabular-nums transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring ${triggerWidthClass}`}
		>
			{triggerLabel}
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
						aria-label={previousPickerLabel}
						class="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
						onclick={movePickerBackward}
					>
						<ChevronLeftIcon class="size-3.5" />
					</button>
					<button
						type="button"
						class="flex-1 rounded py-1 text-xs font-semibold tabular-nums hover:bg-accent"
						onclick={togglePickerMode}
					>
						{#if pickerMode === 'month'}
							{pickerYear}
						{:else}
							{pickerYearWindowStart}–{pickerYearWindowStart + 11}
						{/if}
					</button>
					<button
						type="button"
						aria-label={nextPickerLabel}
						class="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
						onclick={movePickerForward}
					>
						<ChevronRightIcon class="size-3.5" />
					</button>
				</header>
				{#if pickerMode === 'month'}
					<div class="grid grid-cols-3 gap-1">
						{#each monthShortLabels as label, index (index)}
							<button
								type="button"
								class="flex h-8 items-center justify-center rounded text-xs tabular-nums transition-colors {pickerYear === selectedYearMonth.year && index === selectedYearMonth.month - 1
									? 'bg-primary font-semibold text-primary-foreground'
									: 'text-foreground hover:bg-accent'}"
								onclick={() => selectMonth(index)}
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
									: 'text-foreground hover:bg-accent'}"
								onclick={() => selectYear(year)}
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
		class={`flex ${navigationButtonClass} items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring`}
		onclick={() => moveSelectedMonth(1)}
	>
		<ChevronRightIcon class="size-4" />
	</button>
</div>

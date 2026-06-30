	<script lang="ts">
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
		close: () => void;
	};

	type PickerMode = 'month' | 'year';

	let { selectedDate, localeCode, text, selectDate, close }: CalendarToolbarDatePickerProps = $props();

	let pickerMode = $state<PickerMode>('month');
	let pickerYear = $state(initialPickerYear());
	let pickerYearWindowStart = $state(yearWindowStart(initialPickerYear()));
	let monthLabels = $derived(monthLabelsForLocale(localeCode));
	let yearOptions = $derived(Array.from({ length: 12 }, (_, index) => pickerYearWindowStart + index));

	function selectMonth(monthIndex: number): void {
		const selectedDay = Math.min(selectedDate.getDate(), daysInMonth(pickerYear, monthIndex));
		selectDate(new Date(pickerYear, monthIndex, selectedDay, 12, 0, 0, 0));
		close();
	}

	function selectYear(year: number): void {
		pickerYear = year;
		pickerMode = 'month';
	}

	function shiftPickerPage(delta: number): void {
		if (pickerMode === 'month') {
			pickerYear += delta;
			pickerYearWindowStart = yearWindowStart(pickerYear);
			return;
		}
		pickerYearWindowStart += delta * 12;
	}

	function togglePickerMode(): void {
		pickerMode = pickerMode === 'month' ? 'year' : 'month';
		pickerYearWindowStart = yearWindowStart(pickerYear);
	}

	function pickerPageLabel(): string {
		if (pickerMode === 'month') return String(pickerYear);
		return `${pickerYearWindowStart}-${pickerYearWindowStart + 11}`;
	}

	function previousLabel(): string {
		return pickerMode === 'month' ? text.previousYear : text.previousTwelveYears;
	}

	function nextLabel(): string {
		return pickerMode === 'month' ? text.nextYear : text.nextTwelveYears;
	}

	function isSelectedMonth(monthIndex: number): boolean {
		return selectedDate.getFullYear() === pickerYear && selectedDate.getMonth() === monthIndex;
	}

	function isSelectedYear(year: number): boolean {
		return selectedDate.getFullYear() === year;
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key !== 'Escape') return;
		event.stopPropagation();
		close();
	}

	function stopDialogClickPropagation(event: MouseEvent): void {
		event.stopPropagation();
	}

	function initialPickerYear(): number {
		return selectedDate.getFullYear();
	}

	function monthLabelsForLocale(locale: string): string[] {
		return Array.from({ length: 12 }, (_, monthIndex) =>
			new Date(2000, monthIndex, 1).toLocaleDateString(locale, { month: 'short' })
		);
	}

	function daysInMonth(year: number, monthIndex: number): number {
		return new Date(year, monthIndex + 1, 0).getDate();
	}

	function yearWindowStart(year: number): number {
		return Math.floor(year / 12) * 12;
	}
</script>

<dialog
	open
	class="calendar-toolbar-date-picker"
	aria-label={text.pickMonthAndYear}
	onclick={stopDialogClickPropagation}
	onkeydown={handleKeydown}
>
		<header class="calendar-toolbar-date-picker-header">
			<button type="button" class="date-picker-icon-button" aria-label={previousLabel()} onclick={() => shiftPickerPage(-1)}>
				<ChevronLeftIcon class="size-4" />
			</button>
			<button type="button" class="date-picker-page-button" onclick={togglePickerMode}>
				{pickerPageLabel()}
			</button>
			<button type="button" class="date-picker-icon-button" aria-label={nextLabel()} onclick={() => shiftPickerPage(1)}>
				<ChevronRightIcon class="size-4" />
			</button>
		</header>

	{#if pickerMode === 'month'}
		<div class="date-picker-grid date-picker-grid-months">
			{#each monthLabels as monthLabel, monthIndex}
				<button
					type="button"
					class="date-picker-option"
					class:date-picker-option-selected={isSelectedMonth(monthIndex)}
					onclick={() => selectMonth(monthIndex)}
				>
					{monthLabel}
				</button>
			{/each}
		</div>
	{:else}
		<div class="date-picker-grid date-picker-grid-years">
			{#each yearOptions as year}
				<button
					type="button"
					class="date-picker-option"
					class:date-picker-option-selected={isSelectedYear(year)}
					onclick={() => selectYear(year)}
				>
					{year}
				</button>
			{/each}
		</div>
	{/if}
</dialog>

<style>
	.calendar-toolbar-date-picker {
		position: absolute;
		z-index: 50;
		top: calc(100% + 8px);
		left: 50%;
		width: 244px;
		transform: translateX(-50%);
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		background: #ffffff;
		margin: 0;
		padding: 12px;
		box-shadow: 0 16px 40px rgb(15 23 42 / 0.16);
	}

	.calendar-toolbar-date-picker-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 10px;
		gap: 8px;
	}

	.date-picker-icon-button,
	.date-picker-page-button,
	.date-picker-option {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: #18181b;
		font-weight: 700;
		transition:
			background-color 120ms ease,
			color 120ms ease;
	}

		.date-picker-icon-button {
			width: 32px;
			height: 32px;
		}

	.date-picker-page-button {
		height: 32px;
		min-width: 112px;
		padding: 0 12px;
		font-size: 14px;
	}

	.date-picker-grid {
		display: grid;
		gap: 6px;
	}

	.date-picker-grid-months,
	.date-picker-grid-years {
		grid-template-columns: repeat(3, minmax(0, 1fr));
	}

	.date-picker-option {
		height: 34px;
		font-size: 13px;
	}

	.date-picker-icon-button:hover,
	.date-picker-page-button:hover,
	.date-picker-option:hover {
		background: #f4f4f5;
	}

	.date-picker-option-selected {
		background: oklch(0.55 0.19 255);
		color: #ffffff;
	}

	.date-picker-option-selected:hover {
		background: color-mix(in oklch, oklch(0.55 0.19 255) 88%, black);
	}

	:global(html.dark) .calendar-toolbar-date-picker {
		border-color: #27272a;
		background: #09090b;
		box-shadow: 0 16px 40px rgb(0 0 0 / 0.42);
	}

	:global(html.dark) .date-picker-icon-button,
	:global(html.dark) .date-picker-page-button,
	:global(html.dark) .date-picker-option {
		color: #f4f4f5;
	}

	:global(html.dark) .date-picker-icon-button:hover,
	:global(html.dark) .date-picker-page-button:hover,
	:global(html.dark) .date-picker-option:hover {
		background: #18181b;
	}

	:global(html.dark) .date-picker-option-selected {
		background: oklch(0.6 0.2 255);
		color: #ffffff;
	}

	@media (max-width: 767px) {
		.calendar-toolbar-date-picker {
			left: 0;
			width: min(244px, calc(100vw - 20px));
			transform: none;
		}
	}
</style>

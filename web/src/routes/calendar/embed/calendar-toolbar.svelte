<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { calendarText } from '../text';
	import { dateKeyFromDate } from './calendar-month-selection';
	import type { CalendarSearchResult } from './calendar-search';
	import CalendarSearchBox from './calendar-search-box.svelte';
	import CalendarToolbarDatePicker from './calendar-toolbar-date-picker.svelte';
	import CalendarViewSwitcher from './calendar-view-switcher.svelte';

	type CalendarToolbarProps = {
		currentMonthTitle: string;
		toolbarDate: Date;
		searchText: string;
		searchResults: CalendarSearchResult[];
		toolbarView: ViewType;
		localeCode: string;
		changeCalendarView: (viewType: ViewType) => void;
		goToPrevious: () => void;
		goToNext: () => void;
		navigateToDateKey: (dateKey: string) => void;
		navigateToSearchResult: (result: CalendarSearchResult) => void;
		createQuickEvent: (event: MouseEvent) => void;
		openSettings: () => void;
		refreshCalendar: () => void;
	};

	let {
		currentMonthTitle,
		toolbarDate,
		searchText = $bindable(''),
		searchResults,
		toolbarView,
		localeCode,
		changeCalendarView,
		goToPrevious,
		goToNext,
		navigateToDateKey,
		navigateToSearchResult,
		createQuickEvent,
		openSettings,
		refreshCalendar
	}: CalendarToolbarProps = $props();

	const text = createPageText(calendarText);
	let isDatePickerOpen = $state(false);

	function toggleDatePicker(event: MouseEvent): void {
		event.stopPropagation();
		isDatePickerOpen = !isDatePickerOpen;
	}

	function closeDatePicker(): void {
		isDatePickerOpen = false;
	}

	function navigatePrevious(event: MouseEvent): void {
		event.stopPropagation();
		closeDatePicker();
		goToPrevious();
	}

	function navigateNext(event: MouseEvent): void {
		event.stopPropagation();
		closeDatePicker();
		goToNext();
	}

	function selectPickerDate(date: Date): void {
		navigateToDateKey(dateKeyFromDate(date));
	}

	function handleWindowClick(): void {
		if (!isDatePickerOpen) return;
		closeDatePicker();
	}

	function handleWindowKeydown(event: KeyboardEvent): void {
		if (!isDatePickerOpen) return;
		if (event.key !== 'Escape') return;
		closeDatePicker();
	}
</script>

<svelte:window onclick={handleWindowClick} onkeydown={handleWindowKeydown} />

<header class="calendar-toolbar">
	<div class="calendar-toolbar-left">
		<div class="calendar-date-navigation">
			<button type="button" class="toolbar-icon-button" aria-label={text.previous} onclick={navigatePrevious}>
				<ChevronLeftIcon class="size-4" />
			</button>
			<button
				type="button"
				class="calendar-toolbar-title"
				aria-haspopup="dialog"
				aria-expanded={isDatePickerOpen}
				onclick={toggleDatePicker}
			>
				{currentMonthTitle}
			</button>
			<button type="button" class="toolbar-icon-button" aria-label={text.next} onclick={navigateNext}>
				<ChevronRightIcon class="size-4" />
			</button>
			{#if isDatePickerOpen}
				<CalendarToolbarDatePicker
					selectedDate={toolbarDate}
					{localeCode}
					{text}
					selectDate={selectPickerDate}
					close={closeDatePicker}
				/>
			{/if}
		</div>
	</div>
	<div class="calendar-toolbar-actions">
		<button type="button" class="toolbar-icon-button" aria-label={text.refresh} onclick={refreshCalendar}>
			<RefreshCwIcon class="size-4" />
		</button>
		<button type="button" class="toolbar-button settings-button" onclick={openSettings}>
			{text.settings}
		</button>
	</div>
	<CalendarSearchBox bind:searchText {searchResults} {navigateToSearchResult} />
	<CalendarViewSwitcher {toolbarView} {changeCalendarView} />
	<button type="button" class="new-event-button" onclick={(event) => createQuickEvent(event)}>
		<PlusIcon class="size-4" />
		<span>{text.new}</span>
	</button>
</header>

<style>
	.calendar-toolbar {
		display: flex;
		height: 56px;
		flex-shrink: 0;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid #e1e5eb;
		background: #ffffff;
		padding: 0 16px;
	}

	.calendar-toolbar-left {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 12px;
	}

	.calendar-date-navigation {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: 12px;
	}

	.toolbar-button,
	.toolbar-icon-button,
	.new-event-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		border: 1px solid #e5e7eb;
		background: #ffffff;
		color: #111827;
		font-weight: 600;
		white-space: nowrap;
		transition:
			background-color 120ms ease,
			border-color 120ms ease,
			color 120ms ease,
			box-shadow 120ms ease;
	}

	.toolbar-button:hover,
	.toolbar-icon-button:hover {
		background: #f4f4f5;
	}

	.settings-button {
		height: 36px;
		min-width: 56px;
		border-radius: 8px;
		padding: 0 12px;
		font-size: 14px;
	}

	.toolbar-icon-button {
		width: 32px;
		height: 32px;
		border-color: transparent;
		border-radius: 8px;
	}

	.calendar-toolbar-title {
		display: inline-flex;
		height: 36px;
		align-items: center;
		justify-content: center;
		margin: 0;
		border: 0;
		border-radius: 8px;
		background: transparent;
		padding: 0 8px;
		color: #111827;
		white-space: nowrap;
		font-size: 20px;
		font-weight: 800;
		line-height: 1;
		letter-spacing: 0;
	}

	.calendar-toolbar-title:hover {
		background: #f4f4f5;
	}

	.calendar-toolbar-actions {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		gap: 4px;
		margin-left: auto;
	}

	.new-event-button {
		height: 36px;
		gap: 8px;
		border-color: transparent;
		border-radius: 8px;
		background: oklch(0.55 0.19 255);
		padding: 0 14px;
		color: #ffffff;
		font-size: 14px;
	}

	.new-event-button:hover {
		background: color-mix(in oklch, oklch(0.55 0.19 255) 88%, black);
	}

	:global(html.dark) .calendar-toolbar {
		border-bottom-color: #27272a;
		background: #09090b;
	}

	:global(html.dark) .toolbar-button,
	:global(html.dark) .toolbar-icon-button,
	:global(html.dark) .calendar-toolbar-title {
		border-color: #27272a;
		background: #09090b;
		color: #f4f4f5;
	}

	:global(html.dark) .toolbar-button:hover,
	:global(html.dark) .toolbar-icon-button:hover,
	:global(html.dark) .calendar-toolbar-title:hover {
		background: #18181b;
	}

	@media (max-width: 767px) {
		.calendar-toolbar {
			height: auto;
			flex-wrap: wrap;
			gap: 8px;
			padding: 10px;
		}

		.calendar-toolbar-title {
			font-size: 16px;
		}

		.calendar-toolbar-actions {
			margin-left: 0;
			order: 2;
		}

		.new-event-button {
			margin-left: auto;
			order: 3;
		}
	}
</style>

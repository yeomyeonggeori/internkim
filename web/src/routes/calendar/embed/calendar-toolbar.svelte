<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
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
		openSettings
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
			<Button variant="ghost" size="icon-sm" aria-label={text.previous} onclick={navigatePrevious}>
				<ChevronLeftIcon />
			</Button>
			<Button
				variant="ghost"
				class="calendar-toolbar-title"
				aria-haspopup="dialog"
				aria-expanded={isDatePickerOpen}
				onclick={toggleDatePicker}
			>
				{currentMonthTitle}
			</Button>
			<Button variant="ghost" size="icon-sm" aria-label={text.next} onclick={navigateNext}>
				<ChevronRightIcon />
			</Button>
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
		<Button variant="outline" size="sm" class="shrink-0" onclick={openSettings}>
			{text.settings}
		</Button>
	</div>
	<div class="calendar-toolbar-search-row">
		<CalendarSearchBox bind:searchText {searchResults} {navigateToSearchResult} />
		<Button size="sm" class="mobile-new-event-button shrink-0" aria-label={text.new} onclick={(event) => createQuickEvent(event)}>
			<PlusIcon />
			{text.new}
		</Button>
	</div>
	<CalendarViewSwitcher {toolbarView} {changeCalendarView} />
	<Button size="sm" class="desktop-new-event-button shrink-0" onclick={(event) => createQuickEvent(event)}>
		<PlusIcon />
		{text.new}
	</Button>
</header>

<style>
	.calendar-toolbar {
		display: flex;
		height: 56px;
		flex-shrink: 0;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid var(--color-border);
		background: var(--color-background);
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
		gap: 4px;
	}

	:global(.calendar-toolbar-title) {
		font-size: 20px;
		font-weight: 700;
	}

	.calendar-toolbar-actions {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		gap: 4px;
		margin-left: auto;
	}

	.calendar-toolbar-left {
		flex-shrink: 0;
	}

	.calendar-toolbar-search-row {
		display: contents;
	}

	:global(.mobile-new-event-button) {
		display: none;
	}

	@media (max-width: 767px) {
		.calendar-toolbar {
			height: auto;
			flex-wrap: wrap;
			gap: 8px;
			padding: 10px;
		}

		:global(.calendar-toolbar-title) {
			font-size: 16px;
		}

		.calendar-toolbar-actions {
			margin-left: auto;
			order: 2;
		}

		.calendar-toolbar-search-row {
			display: flex;
			width: 100%;
			order: 4;
			align-items: center;
			gap: 8px;
		}

		.calendar-toolbar-search-row :global(.calendar-search-shell) {
			flex: 1 1 auto;
			order: 0;
			width: auto;
			min-width: 0;
			margin-left: 0;
		}

		:global(.desktop-new-event-button) {
			display: none;
		}

		:global(.mobile-new-event-button) {
			display: inline-flex;
			margin-left: auto;
		}
	}
</style>

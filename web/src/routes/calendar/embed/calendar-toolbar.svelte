<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { calendarText } from '../text';
	import type { CalendarSearchResult } from './calendar-search';
	import CalendarSearchBox from './calendar-search-box.svelte';
	import CalendarViewSwitcher from './calendar-view-switcher.svelte';

	type CalendarToolbarProps = {
		currentMonthTitle: string;
		searchText: string;
		searchResults: CalendarSearchResult[];
		toolbarView: ViewType;
		changeCalendarView: (viewType: ViewType) => void;
		goToToday: () => void;
		goToPrevious: () => void;
		goToNext: () => void;
		navigateToSearchResult: (result: CalendarSearchResult) => void;
		createQuickEvent: (event: MouseEvent) => void;
	};

	let {
		currentMonthTitle,
		searchText = $bindable(''),
		searchResults,
		toolbarView,
		changeCalendarView,
		goToToday,
		goToPrevious,
		goToNext,
		navigateToSearchResult,
		createQuickEvent
	}: CalendarToolbarProps = $props();

	const text = createPageText(calendarText);
</script>

<header class="calendar-toolbar">
	<div class="calendar-toolbar-left">
		<button type="button" class="toolbar-button today-button" onclick={goToToday}>
			{text.today}
		</button>
		<button type="button" class="toolbar-icon-button" aria-label={text.previous} onclick={goToPrevious}>
			<ChevronLeftIcon class="size-4" />
		</button>
		<button type="button" class="toolbar-icon-button" aria-label={text.next} onclick={goToNext}>
			<ChevronRightIcon class="size-4" />
		</button>
		<h1 class="calendar-toolbar-title">{currentMonthTitle}</h1>
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

	.today-button {
		height: 36px;
		min-width: 94px;
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
		margin: 0 4px;
		white-space: nowrap;
		font-size: 20px;
		font-weight: 800;
		line-height: 1;
		letter-spacing: 0;
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
	:global(html.dark) .toolbar-icon-button {
		border-color: #27272a;
		background: #09090b;
		color: #f4f4f5;
	}

	:global(html.dark) .toolbar-button:hover,
	:global(html.dark) .toolbar-icon-button:hover {
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
			margin-left: 2px;
			font-size: 16px;
		}

		.new-event-button {
			margin-left: auto;
		}
	}
</style>

<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import { calendarText } from '../text';

	type CalendarViewSwitcherProps = {
		toolbarView: ViewType;
		changeCalendarView: (viewType: ViewType) => void;
	};

	let { toolbarView, changeCalendarView }: CalendarViewSwitcherProps = $props();
	const text = createPageText(calendarText);
</script>

<div class="calendar-view-switcher" aria-label={text.calendarView}>
	<button type="button" class:active-view={toolbarView === ViewType.DAY} onclick={() => changeCalendarView(ViewType.DAY)}>{text.day}</button>
	<button type="button" class:active-view={toolbarView === ViewType.WEEK} onclick={() => changeCalendarView(ViewType.WEEK)}>{text.week}</button>
	<button type="button" class:active-view={toolbarView === ViewType.MONTH} onclick={() => changeCalendarView(ViewType.MONTH)}>{text.month}</button>
</div>

<style>
	.calendar-view-switcher {
		display: inline-flex;
		height: 36px;
		align-items: center;
		border-radius: 8px;
		background: #f4f4f5;
		padding: 2px;
	}

	.calendar-view-switcher button {
		display: inline-flex;
		height: 32px;
		min-width: 58px;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: 7px;
		background: transparent;
		padding: 0 8px;
		color: #71717a;
		font-size: 14px;
		font-weight: 600;
		white-space: nowrap;
		transition:
			background-color 120ms ease,
			border-color 120ms ease,
			color 120ms ease,
			box-shadow 120ms ease;
	}

	.calendar-view-switcher button:hover {
		background: #f4f4f5;
	}

	.calendar-view-switcher button.active-view {
		background: #ffffff;
		color: #111827;
		box-shadow: 0 1px 4px rgb(15 23 42 / 0.14);
	}

	:global(html.dark) .calendar-view-switcher {
		background: #18181b;
	}

	:global(html.dark) .calendar-view-switcher button {
		border-color: transparent;
		background: transparent;
		color: #a1a1aa;
	}

	:global(html.dark) .calendar-view-switcher button:hover {
		background: #18181b;
	}

	:global(html.dark) .calendar-view-switcher button.active-view {
		background: #27272a;
		color: #f4f4f5;
		box-shadow: none;
	}
</style>

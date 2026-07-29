<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Popover from '$lib/components/ui/popover';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import { calendarText } from '../text';
	import { dateKeyFromDate } from './calendar-month-selection';
	import CalendarToolbarDatePicker from './calendar-toolbar-date-picker.svelte';
	import CalendarViewSwitcher from './calendar-view-switcher.svelte';

	type CalendarToolbarProps = {
		currentMonthTitle: string;
		toolbarDate: Date;
		toolbarView: ViewType;
		localeCode: string;
		changeCalendarView: (viewType: ViewType) => void;
		goToPrevious: () => void;
		goToToday: () => void;
		goToNext: () => void;
		navigateToDateKey: (dateKey: string) => void;
		createQuickEvent: (event: MouseEvent) => void;
		openSettings: () => void;
	};

	let {
		currentMonthTitle,
		toolbarDate,
		toolbarView,
		localeCode,
		changeCalendarView,
		goToPrevious,
		goToToday,
		goToNext,
		navigateToDateKey,
		createQuickEvent,
		openSettings
	}: CalendarToolbarProps = $props();

	const text = createPageText(calendarText);
	const stepLabels = $derived(navigationLabels(toolbarView));
	let isDatePickerOpen = $state(false);

	function navigationLabels(viewType: ViewType): { previous: string; next: string } {
		if (viewType === ViewType.DAY) return { previous: text.previousDay, next: text.nextDay };
		if (viewType === ViewType.WEEK) return { previous: text.previousWeek, next: text.nextWeek };
		return { previous: text.previousMonth, next: text.nextMonth };
	}

	function selectPickerDate(date: Date): void {
		isDatePickerOpen = false;
		navigateToDateKey(dateKeyFromDate(date));
	}
</script>

<Tooltip.Provider delayDuration={120}>
<header class="bg-background flex min-h-14 flex-wrap items-center gap-2 border-b px-4 py-2">
	<Popover.Root bind:open={isDatePickerOpen}>
		<Popover.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="ghost" class="-ml-2 gap-1.5 px-2 text-lg font-semibold tabular-nums">
					{currentMonthTitle}
					<ChevronDownIcon class="text-muted-foreground size-4" />
				</Button>
			{/snippet}
		</Popover.Trigger>
		<Popover.Content align="start" class="w-64" onOpenAutoFocus={(event) => event.preventDefault()}>
			<CalendarToolbarDatePicker selectedDate={toolbarDate} {localeCode} {text} selectDate={selectPickerDate} />
		</Popover.Content>
	</Popover.Root>

	<div class="ml-auto flex flex-wrap items-center justify-end gap-2">
		<CalendarViewSwitcher {toolbarView} {changeCalendarView} />
		<ButtonGroup.Root>
			<TooltipIconButton label={stepLabels.previous} variant="outline" size="icon-sm" onclick={goToPrevious}>
				<ChevronLeftIcon />
			</TooltipIconButton>
			<Button variant="outline" size="sm" onclick={goToToday}>{text.today}</Button>
			<TooltipIconButton label={stepLabels.next} variant="outline" size="icon-sm" onclick={goToNext}>
				<ChevronRightIcon />
			</TooltipIconButton>
		</ButtonGroup.Root>
		<TooltipIconButton label={text.settings} variant="outline" size="icon-sm" onclick={openSettings}>
			<SettingsIcon />
		</TooltipIconButton>
		<Button size="sm" onclick={(event) => createQuickEvent(event)}>
			<PlusIcon />
			<span class="max-sm:sr-only">{text.new}</span>
		</Button>
	</div>
</header>
</Tooltip.Provider>

<script lang="ts">
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Popover from '$lib/components/ui/popover';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '../calendar-view-type';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
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
		openSettings: () => void;
		participantOptions: CalendarParticipantFilterOption[];
		participantFilterKey: string;
		selectParticipantFilter: (participantKey: string) => void;
	};

	type CalendarParticipantFilterOption = {
		value: string;
		label: string;
		email: string;
		image: string;
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
		openSettings,
		participantOptions,
		participantFilterKey,
		selectParticipantFilter
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
<header class="bg-background border-border/50 sticky top-0 z-20 flex min-h-14 flex-wrap items-center gap-2 border-b py-2 pr-4 pl-6">
	<Popover.Root bind:open={isDatePickerOpen}>
		<Popover.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="ghost" class="-ml-2 gap-1.5 px-2 text-[22px] leading-none font-extrabold tabular-nums">
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
		<FilterCombobox
			value={participantFilterKey}
			options={participantOptions}
			label={text.filterParticipant}
			searchPlaceholder={text.draftPopover.participantsPlaceholder}
			onSelect={selectParticipantFilter}
			class="h-8 w-44"
		>
			{#snippet selectedContent(option)}
				<PersonAvatar
					name={option.label}
					email={option.email}
					seed={option.value}
					image={option.image}
					class="size-5 shrink-0"
				/>
				<span class="truncate">{option.label}</span>
			{/snippet}
			{#snippet optionContent(option)}
				{#if option.value}
					<PersonAvatar
						name={option.label}
						email={option.email}
						seed={option.value}
						image={option.image}
						class="size-6"
					/>
				{/if}
				<span class="truncate">{option.label}</span>
			{/snippet}
		</FilterCombobox>
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
	</div>
</header>
</Tooltip.Provider>

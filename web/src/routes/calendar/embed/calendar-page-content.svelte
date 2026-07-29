<script lang="ts">
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import type { useCalendarApp, ViewType } from '@dayflow/svelte';
	import type { CalendarLocaleText } from '../text';
	import type { CalendarConflict } from './calendar-conflicts';
	import type { CalendarParticipant } from './calendar-participants';
	import CalendarConflictBanner from './calendar-conflict-banner.svelte';
	import type { DraftPopoverAnchor, DraftPopoverState } from './calendar-draft-popover-state';
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';
	import CalendarPageDraftPopover from './calendar-page-draft-popover.svelte';
	import type { CalendarSearchResult } from './calendar-search';
	import CalendarStage from './calendar-stage.svelte';
	import CalendarToolbar from './calendar-toolbar.svelte';
	import type { TimelineRangePreviewSegment } from './calendar-timeline-preview';

	type CalendarOption = {
		id: string;
		name: string;
	};

	type CalendarPageContentProps = {
		auditEvent: Pick<DayFlowEvent, 'meta'> | null;
		activeMobileEditorEventID: string | null;
		calendar: ReturnType<typeof useCalendarApp>;
		calendarOptions: CalendarOption[];
		participantCandidates: CalendarParticipant[];
		conflicts: CalendarConflict[];
		createQuickEvent: (event: MouseEvent) => void;
		currentMonthTitle: string;
		deletePopover: () => void;
		dismissConflict: (conflictID: number) => void | Promise<void>;
		goToNext: () => void;
		goToPrevious: () => void;
		goToToday: () => void;
		isMobileTwoDayWeekView: boolean;
		localeCode: string;
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		navigateToDateKey: (dateKey: string) => void;
		selectedMonthDateKey: string | null;
		visibleMonthChanged: (month: Date) => void;
		addEventOnDay: (dateKey: string) => void;
		addEventOnRange: (startDateKey: string, endDateKey: string) => void;
		addEventOnTimeRange: (start: Date, end: Date) => void;
		deleteEvent: (eventID: string) => void;
		openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
		openSettings: () => void;
		popover: DraftPopoverState | null;
		refreshConflicts: () => void;
		saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
		savePopover: () => void;
		cancelPopover: () => void;
		selectDate: (dateKey: string) => void;
		selectedEventID: string | null;
		stageEvents: DayFlowEvent[];
		text: CalendarLocaleText;
		timelineRangePreviewSegments: TimelineRangePreviewSegment[];
		timelineRangePreviewTitle: string;
		toolbarDate: Date;
		toolbarView: ViewType;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
		changeCalendarView: (view: ViewType) => void;
		clearSelectedEvent: () => void;
		clearActiveMobileEditorEvent: (eventID: string) => void;
		stageElement?: HTMLElement | null;
	};

	let {
		auditEvent,
		activeMobileEditorEventID,
		calendar,
		calendarOptions,
		participantCandidates,
		cancelPopover,
		changeCalendarView,
		clearSelectedEvent,
		clearActiveMobileEditorEvent,
		conflicts,
		createQuickEvent,
		currentMonthTitle,
		deletePopover,
		dismissConflict,
		goToNext,
		goToPrevious,
		goToToday,
		isMobileTwoDayWeekView,
		localeCode,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		navigateToDateKey,
		selectedMonthDateKey,
		visibleMonthChanged,
		addEventOnDay,
		addEventOnRange,
		addEventOnTimeRange,
		deleteEvent,
		openEvent,
		openSettings,
		popover,
		refreshConflicts,
		saveMovedEvent,
		savePopover,
		selectDate,
		selectedEventID,
		stageEvents,
		text,
		timelineRangePreviewSegments,
		timelineRangePreviewTitle,
		toolbarDate,
		toolbarView,
		updatePopover,
		stageElement = $bindable<HTMLElement | null>(null)
	}: CalendarPageContentProps = $props();
</script>

<main class="calendar-page flex min-h-screen flex-col">
	<CalendarConflictBanner
		conflicts={conflicts}
		{dismissConflict}
		refreshConflicts={refreshConflicts}
	/>
	<CalendarToolbar
		{currentMonthTitle}
		{toolbarDate}
		{toolbarView}
		{localeCode}
		{changeCalendarView}
		{goToPrevious}
		{goToToday}
		{goToNext}
		{navigateToDateKey}
		createQuickEvent={createQuickEvent}
		{openSettings}
	/>
	<CalendarStage
		{activeMobileEditorEventID}
		{calendar}
		{clearActiveMobileEditorEvent}
		{clearSelectedEvent}
		events={stageEvents}
		{participantCandidates}
		{localeCode}
		{isMobileTwoDayWeekView}
		{text}
		monthMoreText={{
			ariaLabel: text.monthMoreAriaLabel,
			button: text.monthMoreButton
		}}
		{toolbarView}
		{toolbarDate}
		selectedEventID={selectedEventID}
		{openEvent}
		{selectDate}
		{saveMovedEvent}
		bind:stageElement
		{monthRangePreviewSegments}
		{monthRangePreviewTitle}
		editingEventID={popover?.eventID ?? null}
		editingTitle={popover?.title ?? ''}
		{timelineRangePreviewSegments}
		{timelineRangePreviewTitle}
		{navigateToDateKey}
		{selectedMonthDateKey}
		{visibleMonthChanged}
		{addEventOnDay}
		{addEventOnRange}
		{addEventOnTimeRange}
		{deleteEvent}
	/>
	<CalendarPageDraftPopover
		{popover}
		{auditEvent}
		{calendarOptions}
		{participantCandidates}
		{localeCode}
		{text}
		{updatePopover}
		{savePopover}
		{cancelPopover}
		{deletePopover}
	/>
</main>

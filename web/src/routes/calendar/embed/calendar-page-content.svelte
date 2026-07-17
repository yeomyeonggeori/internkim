<script lang="ts">
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import type { useCalendarApp, ViewType } from '@dayflow/svelte';
	import type { CalendarLocaleText } from '../text';
	import type { CalendarConflict } from './calendar-conflicts';
	import type { CalendarParticipant } from './calendar-participants';
	import CalendarConflictBanner from './calendar-conflict-banner.svelte';
	import type { DraftPopoverAnchor, DraftPopoverState } from './calendar-draft-popover-state';
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';
	import type { VisibleMonthScrollLabel } from './calendar-month-scroll-overlay-state';
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
		isSaving: boolean;
		isMobileTwoDayWeekView: boolean;
		localeCode: string;
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		monthScrollOverlayLabels: VisibleMonthScrollLabel[];
		navigateToDateKey: (dateKey: string) => void;
		navigateToSearchResult: (result: CalendarSearchResult) => void;
		openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
		openSettings: () => void;
		popover: DraftPopoverState | null;
		refreshConflicts: () => void;
		refreshCalendar: () => void;
		repositionPopover: (size: { width: number; height: number }) => void;
		saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
		savePopover: () => void;
		cancelPopover: () => void;
		searchResults: CalendarSearchResult[];
		selectEvent: (eventID: string) => void;
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
		searchText?: string;
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
		isSaving,
		isMobileTwoDayWeekView,
		localeCode,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		monthScrollOverlayLabels,
		navigateToDateKey,
		navigateToSearchResult,
		openEvent,
		openSettings,
		popover,
		refreshConflicts,
		refreshCalendar,
		repositionPopover,
		saveMovedEvent,
		savePopover,
		searchResults,
		selectEvent,
		selectDate,
		selectedEventID,
		stageEvents,
		text,
		timelineRangePreviewSegments,
		timelineRangePreviewTitle,
		toolbarDate,
		toolbarView,
		updatePopover,
		searchText = $bindable(''),
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
		bind:searchText
		{searchResults}
		{toolbarView}
		{localeCode}
		{changeCalendarView}
		{goToPrevious}
		{goToNext}
		{navigateToDateKey}
		{navigateToSearchResult}
		createQuickEvent={createQuickEvent}
		{openSettings}
		{refreshCalendar}
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
		{selectEvent}
		{selectDate}
		{saveMovedEvent}
		bind:stageElement
		{monthRangePreviewSegments}
		{monthRangePreviewTitle}
		{timelineRangePreviewSegments}
		{timelineRangePreviewTitle}
		{monthScrollOverlayLabels}
		{navigateToDateKey}
	/>
	<CalendarPageDraftPopover
		{popover}
		{auditEvent}
		{calendarOptions}
		{participantCandidates}
		{isSaving}
		{localeCode}
		{text}
		{updatePopover}
		{repositionPopover}
		{savePopover}
		{cancelPopover}
		{deletePopover}
	/>
</main>

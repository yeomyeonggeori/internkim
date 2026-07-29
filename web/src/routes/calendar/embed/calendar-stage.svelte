<script lang="ts">
	import { setContext } from 'svelte';
	import { DayFlowCalendar, useCalendarApp, ViewType } from '@dayflow/svelte';
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import type { CalendarLocaleText } from '../text';
	import type { CalendarParticipant } from './calendar-participants';
	import CalendarDayFlowEventActivator from './calendar-dayflow-event-activator.svelte';
	import { installCalendarDayFlowEventActivation } from './calendar-dayflow-event-activation';
	import CalendarMobileEventEditor from './calendar-mobile-event-editor.svelte';
	import CalendarMonthEventLayer from './calendar-month-event-layer.svelte';
	import CalendarMonthRangePreview from './calendar-month-range-preview.svelte';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';
	import { dateKeyFromWeekHeaderTarget } from './calendar-month-selection';
	import { calendarMobileTwoDayWeekDateKeyForColumn } from './calendar-mobile-two-day-week';
	import {
		mobileEventEditorActivationContextKey,
		mobileEventEditorLocaleContextKey,
		mobileEventEditorParticipantsContextKey,
		mobileEventEditorPersistenceContextKey,
		type MobileEventEditorActivationContext,
		type MobileEventEditorLocaleContext,
		type MobileEventEditorParticipantsContext,
		type MobileEventEditorPersistenceContext
	} from './calendar-mobile-event-editor-types';
	import CalendarTimelineRangePreview from './calendar-timeline-range-preview.svelte';
	import type { TimelineRangePreviewSegment } from './calendar-timeline-preview';

	type MonthScrollOverlayLabel = {
		id: string;
		text: string;
		top: number;
		direction: 1 | -1;
		isFading: boolean;
	};

	type CalendarStageProps = {
		activeMobileEditorEventID: string | null;
		calendar: ReturnType<typeof useCalendarApp>;
		clearActiveMobileEditorEvent: (eventID: string) => void;
		clearSelectedEvent: () => void;
		events: DayFlowEvent[];
		participantCandidates: CalendarParticipant[];
		isMobileTwoDayWeekView: boolean;
		localeCode: string;
		text: CalendarLocaleText;
		monthMoreText: {
			ariaLabel: string;
			button: string;
		};
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		navigateToDateKey: (dateKey: string) => void;
		openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
		saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
		selectDate: (dateKey: string) => void;
		selectedEventID: string | null;
		timelineRangePreviewSegments: TimelineRangePreviewSegment[];
		timelineRangePreviewTitle: string;
		toolbarView: ViewType;
		toolbarDate: Date;
		stageElement?: HTMLElement | null;
	};

	let {
		activeMobileEditorEventID,
		calendar,
		clearActiveMobileEditorEvent,
		clearSelectedEvent,
		events,
		participantCandidates,
		isMobileTwoDayWeekView,
		localeCode,
		text,
		monthMoreText,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		navigateToDateKey,
		openEvent,
		saveMovedEvent,
		selectDate,
		selectedEventID,
		timelineRangePreviewSegments,
		timelineRangePreviewTitle,
		toolbarView,
		toolbarDate,
		stageElement = $bindable<HTMLElement | null>(null)
	}: CalendarStageProps = $props();

	setContext<MobileEventEditorActivationContext>(mobileEventEditorActivationContextKey, {
		getActiveEventID: () => activeMobileEditorEventID,
		getStageElement: () => stageElement,
		clearActiveEvent: (eventID) => clearActiveMobileEditorEvent(eventID)
	});

	setContext<MobileEventEditorPersistenceContext>(mobileEventEditorPersistenceContextKey, {
		saveEvent: (event) => saveMovedEvent(event)
	});
	setContext<MobileEventEditorLocaleContext>(mobileEventEditorLocaleContextKey, {
		getText: () => text
	});
	setContext<MobileEventEditorParticipantsContext>(mobileEventEditorParticipantsContextKey, {
		getCandidates: () => participantCandidates
	});

	function handleMobileTwoDayWeekDateClick(event: MouseEvent): void {
		if (!isMobileTwoDayWeekView || !stageElement) return;
		if (!(event.target instanceof HTMLElement)) return;
		const dateButton = event.target.closest<HTMLElement>('.df-compact-header-date-button');
		if (!dateButton || !stageElement.contains(dateButton)) return;
		const dateButtons = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-compact-header-date-button'));
		const columnIndex = dateButtons.indexOf(dateButton);
		if (columnIndex < 0) return;
		navigateToDateKey(calendarMobileTwoDayWeekDateKeyForColumn(toolbarDate, columnIndex));
	}

	function handleWeekHeaderDateClick(event: MouseEvent): void {
		if (toolbarView !== ViewType.WEEK || isMobileTwoDayWeekView || !stageElement) return;
		if (!(event.target instanceof Element)) return;
		const dateKey = dateKeyFromWeekHeaderTarget(stageElement, event.target, toolbarDate);
		if (!dateKey) return;
		navigateToDateKey(dateKey);
	}

	function handleStageDateClick(event: MouseEvent): void {
		handleMobileTwoDayWeekDateClick(event);
		handleWeekHeaderDateClick(event);
	}

	$effect(() => {
		const currentStageElement = stageElement;
		if (!currentStageElement) return;
		const stopDayFlowEventActivation = installCalendarDayFlowEventActivation({
			stageElement: currentStageElement,
			openEvent: (eventID, anchor) => openEvent(eventID, anchor)
		});
		currentStageElement.addEventListener('click', handleStageDateClick, true);
		return () => {
			currentStageElement.removeEventListener('click', handleStageDateClick, true);
			stopDayFlowEventActivation();
		};
	});

	import './calendar-stage.css';
</script>

<div
	bind:this={stageElement}
	class="calendar-stage"
	class:calendar-stage-day={toolbarView === ViewType.DAY}
	class:calendar-stage-week={toolbarView === ViewType.WEEK}
	class:calendar-stage-month={toolbarView === ViewType.MONTH}
	class:calendar-stage-mobile-two-day-week={isMobileTwoDayWeekView}
	tabindex="-1"
	role="region"
	aria-label={text.pageTitle}
>
	<DayFlowCalendar
		{calendar}
		eventContentDay={CalendarDayFlowEventActivator}
		eventContentWeek={CalendarDayFlowEventActivator}
		eventContentMonth={CalendarDayFlowEventActivator}
		eventContentAllDayDay={CalendarDayFlowEventActivator}
		eventContentAllDayWeek={CalendarDayFlowEventActivator}
		eventContentAllDayMonth={CalendarDayFlowEventActivator}
		mobileEventDetail={CalendarMobileEventEditor}
	/>

	<CalendarMonthEventLayer
		{clearSelectedEvent}
		{events}
		{localeCode}
		{monthMoreText}
		{openEvent}
		{saveMovedEvent}
		{selectDate}
		{selectedEventID}
		{stageElement}
		{toolbarView}
	/>
	<CalendarMonthRangePreview segments={monthRangePreviewSegments} title={monthRangePreviewTitle} />
	<CalendarTimelineRangePreview segments={timelineRangePreviewSegments} title={timelineRangePreviewTitle} />
</div>

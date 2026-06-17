<script lang="ts">
	import { DayFlowCalendar, useCalendarApp, ViewType } from '@dayflow/svelte';
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import CalendarEventContent from './calendar-event-content.svelte';
	import CalendarMonthEventLayer from './calendar-month-event-layer.svelte';
	import CalendarMonthRangePreview from './calendar-month-range-preview.svelte';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';
	import CalendarMonthScrollOverlay from './calendar-month-scroll-overlay.svelte';
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
		calendar: ReturnType<typeof useCalendarApp>;
		clearSelectedEvent: () => void;
		events: DayFlowEvent[];
		localeCode: string;
		monthMoreText: {
			ariaLabel: string;
			button: string;
		};
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		monthScrollOverlayLabels: MonthScrollOverlayLabel[];
		openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
		saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
		selectEvent: (eventID: string) => void;
		selectedEventID: string | null;
		timelineRangePreviewSegments: TimelineRangePreviewSegment[];
		timelineRangePreviewTitle: string;
		toolbarView: ViewType;
		stageElement?: HTMLElement | null;
	};

	let {
		calendar,
		clearSelectedEvent,
		events,
		localeCode,
		monthMoreText,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		monthScrollOverlayLabels,
		openEvent,
		saveMovedEvent,
		selectEvent,
		selectedEventID,
		timelineRangePreviewSegments,
		timelineRangePreviewTitle,
		toolbarView,
		stageElement = $bindable<HTMLElement | null>(null)
	}: CalendarStageProps = $props();
	import './calendar-stage.css';
</script>

<div
	bind:this={stageElement}
	class="calendar-stage"
	class:calendar-stage-day={toolbarView === ViewType.DAY}
	class:calendar-stage-week={toolbarView === ViewType.WEEK}
	class:calendar-stage-month={toolbarView === ViewType.MONTH}
>
	<DayFlowCalendar
		{calendar}
		eventContentDay={CalendarEventContent}
		eventContentWeek={CalendarEventContent}
		eventContentMonth={CalendarEventContent}
		eventContentAllDayDay={CalendarEventContent}
		eventContentAllDayWeek={CalendarEventContent}
		eventContentAllDayMonth={CalendarEventContent}
	/>

	<CalendarMonthEventLayer
		{clearSelectedEvent}
		{events}
		{localeCode}
		{monthMoreText}
		{openEvent}
		{saveMovedEvent}
		{selectEvent}
		{selectedEventID}
		{stageElement}
		{toolbarView}
	/>
	<CalendarMonthRangePreview segments={monthRangePreviewSegments} title={monthRangePreviewTitle} />
	<CalendarTimelineRangePreview segments={timelineRangePreviewSegments} title={timelineRangePreviewTitle} />
	<CalendarMonthScrollOverlay labels={monthScrollOverlayLabels} />
</div>

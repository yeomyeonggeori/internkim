<script lang="ts">
	import { DayFlowCalendar, useCalendarApp, ViewType } from '@dayflow/svelte';
	import type { CalendarAuditRow } from './calendar-audit';
	import CalendarEventAuditCard from './calendar-event-audit-card.svelte';
	import CalendarMonthRangePreview from './calendar-month-range-preview.svelte';
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
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		monthScrollOverlayLabels: MonthScrollOverlayLabel[];
		timelineRangePreviewSegments: TimelineRangePreviewSegment[];
		timelineRangePreviewTitle: string;
		toolbarView: ViewType;
		auditRows: CalendarAuditRow[];
		auditLabel: string;
		stageElement?: HTMLElement | null;
	};

	let {
		calendar,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		monthScrollOverlayLabels,
		timelineRangePreviewSegments,
		timelineRangePreviewTitle,
		toolbarView,
		auditRows,
		auditLabel,
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
	<DayFlowCalendar {calendar} />

	<CalendarMonthRangePreview segments={monthRangePreviewSegments} title={monthRangePreviewTitle} />
	<CalendarTimelineRangePreview segments={timelineRangePreviewSegments} title={timelineRangePreviewTitle} />
	<CalendarMonthScrollOverlay labels={monthScrollOverlayLabels} />

	<CalendarEventAuditCard rows={auditRows} label={auditLabel} />
</div>

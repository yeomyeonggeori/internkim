<script lang="ts">
	import { DayFlowCalendar, useCalendarApp } from '@dayflow/svelte';
	import type { CalendarAuditRow } from './calendar-audit';
	import CalendarEventAuditCard from './calendar-event-audit-card.svelte';
	import CalendarMonthRangePreview from './calendar-month-range-preview.svelte';
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';

	type CalendarStageProps = {
		calendar: ReturnType<typeof useCalendarApp>;
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		auditRows: CalendarAuditRow[];
		auditLabel: string;
		stageElement?: HTMLElement | null;
	};

	let {
		calendar,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		auditRows,
		auditLabel,
		stageElement = $bindable<HTMLElement | null>(null)
	}: CalendarStageProps = $props();
</script>

<div bind:this={stageElement} class="calendar-stage">
	<DayFlowCalendar {calendar} />

	<CalendarMonthRangePreview segments={monthRangePreviewSegments} title={monthRangePreviewTitle} />

	<CalendarEventAuditCard rows={auditRows} label={auditLabel} />
</div>

<style>
	.calendar-stage {
		--df-color-background: #ffffff;
		--df-color-foreground: #2e2e2e;
		--df-color-hover: #f5f5f5;
		--df-color-border: #e5e5e5;
		--df-color-card: #ffffff;
		--df-color-card-foreground: #2e2e2e;
		--df-color-muted: #f3f4f6;
		--df-color-muted-foreground: #6b7280;
		--df-color-primary: oklch(0.55 0.19 255);
		--df-color-primary-foreground: #ffffff;
		--df-color-secondary: #64748b;
		--df-color-secondary-foreground: #ffffff;
		--df-color-destructive: #d42422;
		--df-color-destructive-foreground: #ffffff;
		--background: 0 0% 100%;
		--foreground: 222.2 84% 4.9%;
		--card: 0 0% 100%;
		--card-foreground: 222.2 84% 4.9%;
		--muted: 210 40% 96.1%;
		--muted-foreground: 215.4 16.3% 46.9%;
		--border: 214.3 31.8% 91.4%;
		--destructive: 0 84.2% 60.2%;
		position: relative;
		min-height: 0;
		flex: 1;
		padding: 0;
		color-scheme: light;
		background: #ffffff;
	}

	.calendar-stage :global(.df-calendar-container) {
		width: 100%;
		--df-calendar-height: calc(100svh - 52px);
		color-scheme: light;
		border-radius: 0;
		border: 0;
	}

	.calendar-stage :global(.draft-empty-title-event) {
		display: none !important;
	}

	.calendar-stage :global(.df-header),
	.calendar-stage :global(.df-view-header),
	.calendar-stage :global(.df-view-header-container),
	.calendar-stage :global(.df-calendar-container .df-view-header),
	.calendar-stage :global(.df-calendar-container .df-view-header-container) {
		display: none !important;
	}

	:global(.df-dialog-container),
	:global(.df-event-detail-panel),
	:global(.df-portal),
	:global(.df-range-picker) {
		--df-color-background: #ffffff;
		--df-color-foreground: #2e2e2e;
		--df-color-hover: #f5f5f5;
		--df-color-border: #e5e5e5;
		--df-color-card: #ffffff;
		--df-color-card-foreground: #2e2e2e;
		--df-color-muted: #f3f4f6;
		--df-color-muted-foreground: #6b7280;
		--df-color-primary: oklch(0.55 0.19 255);
		--df-color-primary-foreground: #ffffff;
		--df-color-secondary: #64748b;
		--df-color-secondary-foreground: #ffffff;
		--df-color-destructive: #d42422;
		--df-color-destructive-foreground: #ffffff;
		color-scheme: light;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-other-month='true']) {
		background: transparent;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-other-month='true'] .df-month-date-number) {
		opacity: 0.38;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-other-month='true'] .df-event) {
		opacity: 0.52 !important;
	}

	.calendar-stage :global(.df-week-grid > .df-day-label:nth-child(1)),
	.calendar-stage :global(.df-week-grid > .df-day-label:nth-child(7)) {
		color: #ef4444;
	}

	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1) .df-month-date-number),
	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7) .df-month-date-number) {
		color: #ef4444;
	}

	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1)),
	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7)) {
		background: #fafafa;
	}

	.calendar-stage :global(.df-month-title) {
		display: none !important;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-today='true'] .df-month-date-number),
	.calendar-stage :global(.df-week-day-header[data-today='true'] .df-week-date-number) {
		display: inline-flex;
		min-width: 22px;
		height: 22px;
		align-items: center;
		justify-content: center;
		border-radius: 9999px;
		background: oklch(0.55 0.19 255);
		color: #ffffff;
		font-weight: 700;
	}

	:global(html.dark) .calendar-stage {
		--df-color-background: #09090b;
		--df-color-foreground: #f4f4f5;
		--df-color-hover: #18181b;
		--df-color-border: #27272a;
		--df-color-card: #09090b;
		--df-color-card-foreground: #f4f4f5;
		--df-color-muted: #18181b;
		--df-color-muted-foreground: #a1a1aa;
		background: #09090b;
		color-scheme: dark;
	}

	:global(html.dark) .calendar-stage :global(.df-calendar-container) {
		color-scheme: dark;
		background: #09090b;
	}

	:global(html.dark) :global(.df-dialog-container),
	:global(html.dark) :global(.df-event-detail-panel),
	:global(html.dark) :global(.df-portal),
	:global(html.dark) :global(.df-range-picker) {
		--df-color-background: #09090b;
		--df-color-foreground: #f4f4f5;
		--df-color-hover: #18181b;
		--df-color-border: #27272a;
		--df-color-card: #09090b;
		--df-color-card-foreground: #f4f4f5;
		--df-color-muted: #18181b;
		--df-color-muted-foreground: #a1a1aa;
		color-scheme: dark;
	}

	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1)),
	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7)) {
		background: #111113;
	}

	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1) .df-month-date-number),
	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7) .df-month-date-number) {
		color: #f87171;
	}

	:global(html.dark) .calendar-stage :global(.df-month-day-cell-surface[data-other-month='true']) {
		background: transparent;
	}

	.calendar-stage :global(.df-month-day-cell.month-selected-date) {
		background: color-mix(in oklab, var(--primary) 8%, transparent);
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--primary) 28%, transparent);
	}

	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell.month-selected-date:nth-child(1)),
	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell.month-selected-date:nth-child(7)) {
		background: color-mix(in oklab, var(--primary) 8%, transparent);
	}

	@media (max-width: 767px) {
		.calendar-stage {
			padding: 0;
		}

		.calendar-stage :global(.df-calendar-container) {
			border-radius: 0;
			--df-calendar-height: calc(100svh - 96px);
		}
	}
</style>

<script lang="ts">
	import type { MonthMorePlacement } from './calendar-month-event-geometry';
	import type { MonthEventSegment } from './calendar-month-event-model';
	import {
		formattedMonthMoreDate,
		monthMoreAriaLabel,
		monthMoreButtonText,
		monthMoreEventDateText,
		monthMorePlacementStyle,
		monthMorePopoverStyle
	} from './calendar-month-event-layer-format';

	type CalendarMonthMoreLayerProps = {
		activeMorePlacement: MonthMorePlacement | null;
		localeCode: string;
		monthMoreText: {
			ariaLabel: string;
			button: string;
		};
		morePlacements: MonthMorePlacement[];
		selectedEventID: string | null;
		handleMoreButtonClick: (event: MouseEvent, placement: MonthMorePlacement) => void;
		handleMoreEventClick: (event: MouseEvent, segment: MonthEventSegment, activeDateKey: string) => void;
	};

	let {
		activeMorePlacement,
		localeCode,
		monthMoreText,
		morePlacements,
		selectedEventID,
		handleMoreButtonClick,
		handleMoreEventClick
	}: CalendarMonthMoreLayerProps = $props();
</script>

{#each morePlacements as placement (placement.id)}
	<button
		type="button"
		class="calendar-month-more-button"
		data-date-key={placement.dateKey}
		style={monthMorePlacementStyle(placement)}
		aria-label={monthMoreAriaLabel(monthMoreText.ariaLabel, placement, localeCode)}
		aria-controls={`calendar-month-more-popover-${placement.dateKey}`}
		aria-expanded={activeMorePlacement?.id === placement.id}
		aria-haspopup="dialog"
		onclick={(event) => handleMoreButtonClick(event, placement)}
	>
		{monthMoreButtonText(monthMoreText.button, placement.count)}
	</button>
{/each}
{#if activeMorePlacement}
	<div
		id={`calendar-month-more-popover-${activeMorePlacement.dateKey}`}
		class="calendar-month-more-popover"
		style={monthMorePopoverStyle(activeMorePlacement)}
		role="dialog"
		aria-labelledby={`calendar-month-more-popover-title-${activeMorePlacement.dateKey}`}
	>
		<div
			id={`calendar-month-more-popover-title-${activeMorePlacement.dateKey}`}
			class="calendar-month-more-popover-title"
		>
			{formattedMonthMoreDate(activeMorePlacement.dateKey, localeCode)}
		</div>
		<div class="calendar-month-more-popover-list">
			{#each activeMorePlacement.hiddenSegments as segment (segment.id)}
				<button
					type="button"
					class="calendar-month-more-popover-event"
					class:internkim-calendar-event-focused={segment.eventID === selectedEventID}
					data-event-id={segment.eventID}
					aria-pressed={segment.eventID === selectedEventID}
					onclick={(event) => handleMoreEventClick(event, segment, activeMorePlacement.dateKey)}
				>
					<span class="calendar-month-more-popover-event-title">{segment.titleOnlyText}</span>
					<span class="calendar-month-more-popover-event-date">{monthMoreEventDateText(segment, localeCode)}</span>
				</button>
			{/each}
		</div>
	</div>
{/if}

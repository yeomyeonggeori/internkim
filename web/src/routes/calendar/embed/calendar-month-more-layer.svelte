<script lang="ts">
	import type { MonthMorePlacement } from './calendar-month-event-geometry';
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
		handleMoreButtonClick: (event: MouseEvent, placement: MonthMorePlacement) => void;
		handleMoreEventClick: (event: MouseEvent, eventID: string) => void;
		handleMoreEventDoubleClick: (event: MouseEvent, eventID: string) => void;
	};

	let {
		activeMorePlacement,
		localeCode,
		monthMoreText,
		morePlacements,
		handleMoreButtonClick,
		handleMoreEventClick,
		handleMoreEventDoubleClick
	}: CalendarMonthMoreLayerProps = $props();
</script>

{#each morePlacements as placement (placement.id)}
	<button
		type="button"
		class="calendar-month-more-button"
		data-date-key={placement.dateKey}
		style={monthMorePlacementStyle(placement)}
		aria-label={monthMoreAriaLabel(monthMoreText.ariaLabel, placement, localeCode)}
		onclick={(event) => handleMoreButtonClick(event, placement)}
	>
		{monthMoreButtonText(monthMoreText.button, placement.count)}
	</button>
{/each}
{#if activeMorePlacement}
	<div class="calendar-month-more-popover" style={monthMorePopoverStyle(activeMorePlacement)}>
		<div class="calendar-month-more-popover-title">{formattedMonthMoreDate(activeMorePlacement.dateKey, localeCode)}</div>
		<div class="calendar-month-more-popover-list">
			{#each activeMorePlacement.hiddenSegments as segment (segment.id)}
				<button
					type="button"
					class="calendar-month-more-popover-event"
					data-event-id={segment.eventID}
					onclick={(event) => handleMoreEventClick(event, segment.eventID)}
					ondblclick={(event) => handleMoreEventDoubleClick(event, segment.eventID)}
				>
					<span class="calendar-month-more-popover-event-title">{segment.titleText}</span>
					<span class="calendar-month-more-popover-event-date">{monthMoreEventDateText(segment, localeCode)}</span>
				</button>
			{/each}
		</div>
	</div>
{/if}

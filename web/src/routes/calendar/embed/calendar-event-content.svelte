<script lang="ts">
	import { temporalToDate, type EventContentSlotArgs } from '@dayflow/core';

	let { event, isAllDay }: EventContentSlotArgs = $props();

	const shouldShowTime = $derived(!isAllDay && !event.allDay);
	const startTime = $derived(formatEventTime(temporalToDate(event.start)));
	const endTime = $derived(formatEventTime(temporalToDate(event.end)));

	function formatEventTime(date: Date): string {
		return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
	}
</script>

<span class:calendar-event-content-timed={shouldShowTime} class="calendar-event-content calendar-month-event-content">
	<span class="calendar-event-title calendar-month-event-title">{event.title}{#if shouldShowTime} {startTime}{/if}</span>
	{#if shouldShowTime}
		<span class="calendar-event-time calendar-month-event-time">{endTime}</span>
	{/if}
</span>

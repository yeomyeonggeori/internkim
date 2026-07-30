<script lang="ts">
	import TimeRangeText from '$lib/components/time-range-text.svelte';
	

	type Props = {
		event: { title: string; allDay?: boolean; start: Date };
		isAllDay?: boolean;
		timeLabel?: string;
	};

	let { event, isAllDay, timeLabel }: Props = $props();

	const shouldShowTime = $derived(Boolean(timeLabel) || (!isAllDay && !event.allDay));
	const startTime = $derived(formatEventTime(new Date(event.start)));
	const displayTime = $derived(timeLabel || startTime);
	const timeRange = $derived(timeRangeFromLabel(timeLabel));

	function formatEventTime(date: Date): string {
		return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
	}

	function timeRangeFromLabel(label: string | undefined): { startTime: string; endTime: string } | undefined {
		const match = /^(\d{2}:\d{2})-(\d{2}:\d{2})$/.exec(label ?? '');
		if (!match) return undefined;
		const [, rangeStartTime = '', rangeEndTime = ''] = match;
		return { startTime: rangeStartTime, endTime: rangeEndTime };
	}
</script>

<span class:calendar-event-content-timed={shouldShowTime} class="calendar-event-content calendar-month-event-content">
	<span class="calendar-event-title calendar-month-event-title">{event.title}</span>
	{#if shouldShowTime}
		<span class="calendar-event-time">
			{#if timeRange}
				<TimeRangeText startTime={timeRange.startTime} endTime={timeRange.endTime} size="inherit" tone="inherit" />
			{:else}
				{displayTime}
			{/if}
		</span>
	{/if}
</span>

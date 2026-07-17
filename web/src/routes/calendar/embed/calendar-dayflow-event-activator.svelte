<script lang="ts">
	import type { EventContentSlotArgs } from '@dayflow/core';
	import CalendarEventContent from './calendar-event-content.svelte';

	let { event, isAllDay, isSelected }: EventContentSlotArgs = $props();
</script>

<span
	class="calendar-dayflow-event-activator"
	role="button"
	tabindex="0"
	aria-pressed={isSelected}
>
	<CalendarEventContent {event} {isAllDay} />
</span>

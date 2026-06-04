<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { attendanceText } from '../text';
	import DayEventRow from './day-event-row.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const dayEvents = $derived(
		attendance.summary && attendance.selectedDate
			? attendance.summary.events.filter(
					(event) =>
						event.email === (attendance.selectedEmail || attendance.summary?.currentUserEmail) &&
						event.localDate === attendance.selectedDate
				)
			: []
	);

	const locations = $derived(attendance.summary?.locations ?? []);

	let expanded = $state<Record<string, boolean>>({});

	function toggle(eventID: string) {
		expanded[eventID] = !expanded[eventID];
	}

	function pickLocation(eventID: string, currentLocationID: string | undefined, newLocationID: string) {
		if (newLocationID === currentLocationID) {
			attendance.confirmClassification(eventID);
		} else {
			attendance.overrideLocation(eventID, newLocationID);
		}
	}

	function skipEvent(eventID: string) {
		attendance.dismissEvent(eventID, 'classification_dismissed');
	}

	function confirmClockOut(eventID: string) {
		attendance.confirmClassification(eventID);
	}
</script>

{#if attendance.selectedDate}
	<Card.Root>
		<Card.Header>
			<Card.Title class="text-sm">{attendance.selectedDate}</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-2 text-xs">
			{#each dayEvents as event (event.id)}
				<DayEventRow
					{event}
					{locations}
					isExpanded={!!expanded[event.id]}
					onToggle={toggle}
					onPickLocation={pickLocation}
					onConfirmClockOut={confirmClockOut}
					onSkip={skipEvent}
				/>
			{/each}
			{#if dayEvents.length === 0}
				<p class="text-muted-foreground">{text.eventNone}</p>
			{/if}
		</Card.Content>
	</Card.Root>
{/if}

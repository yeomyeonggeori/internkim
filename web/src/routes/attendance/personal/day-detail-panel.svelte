<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { absenceLabelText, absencesForDate, hasAbsenceDetails } from '../shared/attendance-absence';
	import { attendanceText } from '../text';
	import DayEventRow from './day-event-row.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const dayEvents = $derived(
		attendance.summary && attendance.selectedDate
			? attendance.summary.events.filter(
					(event) =>
						event.email === attendance.summary?.currentUserEmail &&
						event.localDate === attendance.selectedDate
				)
			: []
	);

	const locations = $derived(attendance.summary?.locations ?? []);
	const dayAbsences = $derived(
		attendance.summary && attendance.selectedDate
			? absencesForDate(
					attendance.summary.absences,
					attendance.selectedDate,
					attendance.summary.currentUserEmail
				)
			: []
	);

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
			{#each dayAbsences as absence (absence.id)}
				<div class="rounded-md border border-info/30 bg-info/10 p-3">
					<p class="font-medium text-info">{absenceLabelText(absence, text)}</p>
					{#if hasAbsenceDetails(absence)}
						<div class="mt-1 space-y-1 text-muted-foreground">
							{#if absence.reason}
								<p>{absence.reason}</p>
							{/if}
							{#if absence.createdBy}
								<p>{text.absenceCreatedByTemplate.replace('{user}', absence.createdBy)}</p>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
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
			{#if dayEvents.length === 0 && dayAbsences.length === 0}
				<p class="text-muted-foreground">{text.eventNone}</p>
			{/if}
		</Card.Content>
	</Card.Root>
{/if}

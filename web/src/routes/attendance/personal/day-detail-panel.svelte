<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { absenceLabelText, absencesForDate, hasAbsenceDetails } from '../shared/attendance-absence';
	import { computeDayEvents } from '../shared/attendance-day-events';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import type { AttendanceWorkSegment } from '../shared/attendance-work-segments';
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
				).sort((first, second) => first.occurredAt.localeCompare(second.occurredAt))
			: []
	);
	const day = $derived(
		attendance.selectedDate ? computeDayEvents(attendance.selectedDate, dayEvents) : undefined
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

	function segmentCountLabel(count: number): string {
		return text.locationSegmentCountTemplate.replace('{count}', String(count));
	}

	function segmentRowClass(segment: AttendanceWorkSegment): string {
		if (segment.isOpen) return 'border-success/40 bg-success/5';
		return 'border-border/40';
	}

	function segmentLocationClass(segment: AttendanceWorkSegment): string {
		if (segment.isOpen) return 'text-success';
		return 'text-foreground';
	}
</script>

{#if attendance.selectedDate}
	<div data-testid="personal-day-detail-panel">
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
				{#if day && day.segments.length > 0}
					<div class="space-y-2 rounded-md border border-border/60 p-3">
						<div class="flex items-center justify-between gap-3">
							<p class="font-medium">{text.workSegments}</p>
							{#if day.segments.length > 1}
								<span class="text-[11px] text-muted-foreground">{segmentCountLabel(day.segments.length)}</span>
							{/if}
						</div>
						<div class="space-y-1.5">
							{#each day.segments as segment (segment.id)}
								<div class={`flex items-center justify-between gap-3 rounded border px-2 py-1.5 ${segmentRowClass(segment)}`}>
									<div class="min-w-0">
										<p class={`flex min-w-0 items-center gap-1.5 font-medium ${segmentLocationClass(segment)}`}>
											<MapPinIcon class="h-3 w-3 shrink-0" />
											<span class="truncate">{segment.locationName ?? segment.locationID ?? text.location}</span>
										</p>
										<p class="tabular-nums text-muted-foreground">
											{segment.startTime}{segment.endTime ? `-${segment.endTime}` : '~'}
										</p>
									</div>
									<span class={segment.isOpen ? 'shrink-0 text-success' : 'shrink-0 text-muted-foreground'}>
										{segment.isOpen ? text.inProgress : formatHoursMinutes(segment.workedMinutes)}
									</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}
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
	</div>
{/if}

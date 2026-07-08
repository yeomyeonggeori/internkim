<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import DayEventRow from '../personal/day-event-row.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { absencesForDate, absenceLabelText, hasAbsenceDetails } from '../shared/attendance-absence';
	import { absenceDisplayClass } from '../shared/color-tokens';
	import DurationText from '../shared/duration-text.svelte';
	import LocationLabel from '../shared/location-label.svelte';
	import type { AttendanceText } from '../text';
	import type { TeamStatusDayDetail } from './team-status-day-detail';

	type Props = {
		text: AttendanceText;
		detail: TeamStatusDayDetail;
	};

	let { text, detail }: Props = $props();
	const attendance = getAttendanceState();
	let expandedEventIDs = $state<Record<string, boolean>>({});
	let deletingAbsenceID = $state('');
	let absenceErrorID = $state('');
	let absenceErrorMessage = $state('');

	const isOwnDay = $derived(detail.email === attendance.summary?.currentUserEmail);
	const ownDayAbsences = $derived(
		isOwnDay && attendance.summary
			? absencesForDate(attendance.summary.absences, detail.day.date, attendance.summary.currentUserEmail)
			: []
	);
	const ownDayEvents = $derived(
		isOwnDay && attendance.summary
			? attendance.summary.events
					.filter((event) => event.email === attendance.summary?.currentUserEmail && event.localDate === detail.day.date)
					.sort((first, second) => first.occurredAt.localeCompare(second.occurredAt))
			: []
	);

	const sectionListClass = 'grid gap-2';
	const blockClass = 'rounded-md border border-border/70 bg-card px-3 py-2 shadow-sm';

	function toggleEventExpanded(eventID: string) {
		expandedEventIDs[eventID] = !expandedEventIDs[eventID];
	}

	async function deleteAbsence(absenceID: string) {
		if (deletingAbsenceID) return;
		deletingAbsenceID = absenceID;
		absenceErrorID = '';
		absenceErrorMessage = '';
		try {
			await attendance.deleteAbsence(absenceID);
		} catch (error) {
			absenceErrorID = absenceID;
			absenceErrorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			deletingAbsenceID = '';
		}
	}
</script>

<div class="grid gap-4" data-testid="team-status-day-detail-content">
	<div class="grid gap-2">
		<div class="flex items-center justify-between gap-3" data-testid="team-status-work-record-header">
			<div class="text-sm font-semibold">{text.workRecords}</div>
			{#if detail.day.durationMinutes !== undefined}
				<DurationText minutes={detail.day.durationMinutes} class="shrink-0 text-sm font-semibold text-muted-foreground" />
			{/if}
		</div>
		<div class={sectionListClass} data-testid="team-status-work-record-list">
			{#if detail.day.absenceDetail}
				<div class={`rounded-md border border-border/70 px-3 py-2 shadow-sm ${absenceDisplayClass(detail.day.absenceTone ?? 'leave')}`}>
					<div class="flex items-center justify-between gap-3">
						<div class="min-w-0 truncate text-sm font-medium">{detail.day.absenceDetail.label}</div>
						<div class="shrink-0 text-xs font-medium opacity-75">{detail.day.absenceDetail.periodLabel}</div>
					</div>
					{#if detail.day.absenceDetail.reason}
						<div class="mt-1 text-xs opacity-80">{detail.day.absenceDetail.reason}</div>
					{/if}
					{#if detail.day.absenceDetail.createdBy}
						<div class="mt-1 text-xs opacity-70">
							{text.absenceCreatedByTemplate.replace('{user}', detail.day.absenceDetail.createdBy)}
						</div>
					{/if}
				</div>
			{:else if detail.day.segments.length}
				{#each detail.day.segments as segment (segment.id)}
					<div class={`relative flex items-center justify-between gap-3 overflow-hidden ${blockClass}`} data-testid="team-status-day-segment">
						<span
							class="absolute inset-y-0 left-0 w-1"
							style:background-color={segment.locationColor ?? 'var(--color-muted-foreground)'}
							aria-hidden="true"
						></span>
						<div class="min-w-0">
							<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
								<LocationLabel name={segment.locationName} class="max-w-full" />
							</div>
							<div class="mt-0.5 text-xs tabular-nums text-muted-foreground">{segment.timeLabel}</div>
						</div>
						{#if segment.durationMinutes !== undefined}
							<DurationText
								minutes={segment.durationMinutes}
								class={`shrink-0 text-sm font-medium ${segment.isOpen ? 'text-success' : 'text-muted-foreground'}`}
							/>
						{/if}
					</div>
				{/each}
			{:else}
				<div class="rounded-md border border-border/70 bg-muted/30 px-3 py-6 text-center text-sm text-muted-foreground shadow-sm">
					{text.eventNone}
				</div>
			{/if}
		</div>
	</div>

	{#if isOwnDay}
		<div class={sectionListClass} data-testid="personal-day-detail-panel">
			{#each ownDayAbsences as absence (absence.id)}
				<div class="rounded-md border border-info/30 bg-info/10 p-3 text-xs">
					<div class="flex items-center justify-between gap-3">
						<p class="font-medium text-info">{absenceLabelText(absence, text)}</p>
						<Button
							type="button"
							variant="outline"
							size="sm"
							disabled={!!deletingAbsenceID}
							onclick={() => deleteAbsence(absence.id)}
						>
							{text.cancel}
						</Button>
					</div>
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
					{#if absenceErrorMessage && absenceErrorID === absence.id}
						<p class="mt-2 text-destructive">{absenceErrorMessage}</p>
					{/if}
				</div>
			{/each}
			{#each ownDayEvents as event (event.id)}
				<DayEventRow
					{event}
					locations={attendance.summary?.locations ?? []}
					isExpanded={!!expandedEventIDs[event.id]}
					onToggle={toggleEventExpanded}
					onSaveOverride={(eventID, request) => attendance.updateEvent(eventID, request)}
				/>
			{/each}
		</div>
	{/if}

	<div class="grid gap-2">
		<div class="text-sm font-semibold">{text.calendarEvents}</div>
		<div class={sectionListClass} data-testid="team-status-calendar-event-list">
			{#if detail.context.isCalendarEventsLoading}
				<div class="rounded-md border border-border/70 bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground shadow-sm">
					{text.loading}
				</div>
			{:else if detail.context.hasCalendarEventsLoadFailed}
				<div class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-4 text-center text-sm text-destructive shadow-sm">
					{text.calendarEventsLoadFailed}
				</div>
			{:else if detail.context.calendarEvents.length}
				{#each detail.context.calendarEvents as event (event.id)}
					<div class={blockClass} data-testid="team-status-calendar-event">
						<div class="flex min-w-0 items-center justify-between gap-3">
							<div class="min-w-0 truncate text-sm font-medium">{event.title}</div>
							<div class="shrink-0 text-xs tabular-nums text-muted-foreground">{event.timeLabel}</div>
						</div>
						{#if event.location}
							<div class="mt-0.5 truncate text-xs text-muted-foreground">{event.location}</div>
						{/if}
					</div>
				{/each}
			{:else}
				<div class="rounded-md border border-border/70 bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground shadow-sm">
					{text.noCalendarEvents}
				</div>
			{/if}
		</div>
	</div>

	<div class="grid gap-2">
		<div class="text-sm font-semibold">{text.completedWork}</div>
		<div class={sectionListClass} data-testid="team-status-completed-task-list">
			{#if detail.context.isCompletedWorkLoading}
				<div class="rounded-md border border-border/70 bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground shadow-sm">
					{text.loading}
				</div>
			{:else if detail.context.hasCompletedWorkLoadFailed}
				<div class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-4 text-center text-sm text-destructive shadow-sm">
					{text.completedWorkLoadFailed}
				</div>
			{:else if detail.context.completedTasks.length}
				{#each detail.context.completedTasks as task (task.id)}
					<div class={blockClass} data-testid="team-status-completed-task">
						<div class="min-w-0 truncate text-sm font-medium">{task.title}</div>
						<div class="mt-0.5 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
							<span class="shrink-0">{task.ownerName}</span>
							{#if task.collaboratorNames.length}
								<span class="min-w-0 truncate">{task.collaboratorNames.join(', ')}</span>
							{/if}
						</div>
					</div>
				{/each}
			{:else}
				<div class="rounded-md border border-border/70 bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground shadow-sm">
					{text.noCompletedWork}
				</div>
			{/if}
		</div>
	</div>
</div>

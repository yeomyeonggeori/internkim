<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import CheckCircle2Icon from '@lucide/svelte/icons/circle-check-big';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CalendarEventListCard from '../../calendar/embed/calendar-event-list-card.svelte';
	import { calendarParticipantsFromUnknown } from '../../calendar/embed/calendar-participants';
	import FlowTaskBoardCard from '../../flow/flow-task-board-card.svelte';
	import { flowText } from '../../flow/text';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { absencesForDate, absenceLabelText, hasAbsenceDetails } from '../shared/attendance-absence';
	import type { AttendanceText } from '../text';
	import type { TeamStatusDayDetail } from './team-status-day-detail';
	import TeamStatusWorkRecordSection from './team-status-work-record-section.svelte';

	type Props = {
		text: AttendanceText;
		detail: TeamStatusDayDetail;
	};

	let { text, detail }: Props = $props();
	const attendance = getAttendanceState();
	let deletingAbsenceID = $state('');
	let absenceErrorID = $state('');
	let absenceErrorMessage = $state('');
	const sectionCountBadgeClass = 'inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-muted px-1.5 text-xs font-medium tabular-nums text-muted-foreground';

	const isOwnDay = $derived(detail.email === attendance.summary?.currentUserEmail);
	const ownDayAbsences = $derived(
		isOwnDay && attendance.summary
			? absencesForDate(attendance.summary.absences, detail.day.date, attendance.summary.currentUserEmail)
			: []
	);
	const flowBusinessFallback = $derived(
		text.dateLocale === 'ko-KR' ? flowText.ko.task.businessFallback : flowText.en.task.businessFallback
	);

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

	function openCalendarEvent(eventID: string): void {
		void goto(`/calendar/?date=${encodeURIComponent(detail.day.date)}&event=${encodeURIComponent(eventID)}`);
	}

	function openFlowTask(task: TeamStatusDayDetail['context']['completedTasks'][number]['task']): void {
		void goto(`/flow/?task=${encodeURIComponent(task.id)}`);
	}
</script>

<div class="divide-y" data-testid="team-status-day-detail-content">
	<section class="px-5 py-5">
		<TeamStatusWorkRecordSection {text} {detail} {sectionCountBadgeClass} />

		{#if isOwnDay && ownDayAbsences.length}
			<div class="mt-4 grid gap-2 border-t pt-4" data-testid="personal-day-detail-panel">
				{#each ownDayAbsences as absence (absence.id)}
					<div class="rounded-lg bg-info/10 p-3 text-xs">
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0">
								<p class="font-medium text-info">{absenceLabelText(absence, text)}</p>
								{#if hasAbsenceDetails(absence)}
									<div class="mt-1 space-y-1 text-muted-foreground">
										{#if absence.reason}<p>{absence.reason}</p>{/if}
										{#if absence.createdBy}<p>{text.absenceCreatedByTemplate.replace('{user}', absence.createdBy)}</p>{/if}
									</div>
								{/if}
							</div>
							<Button type="button" variant="outline" size="sm" disabled={!!deletingAbsenceID} onclick={() => deleteAbsence(absence.id)}>
								{text.cancel}
							</Button>
						</div>
						{#if absenceErrorMessage && absenceErrorID === absence.id}
							<p class="mt-2 text-destructive">{absenceErrorMessage}</p>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<section class="px-5 py-5">
		<div class="flex items-center gap-2" data-testid="team-status-calendar-header">
			<div class="flex items-center gap-2">
				<CalendarDaysIcon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
				<h3 class="text-sm font-semibold">{text.calendarEvents}</h3>
			</div>
			{#if !detail.context.isCalendarEventsLoading && !detail.context.hasCalendarEventsLoadFailed}
				<span
					class={sectionCountBadgeClass}
					aria-label={`${text.calendarEvents} ${detail.context.calendarEvents.length}`}
					data-slot="section-count-badge"
					data-testid="section-count-badge"
				>
					{detail.context.calendarEvents.length}
				</span>
			{/if}
		</div>

		<div class="mt-4" data-testid="team-status-calendar-event-list">
			{#if detail.context.isCalendarEventsLoading}
				<div class="grid gap-4" aria-label={text.loading}>
					{#each [0, 1] as loadingRow (loadingRow)}
						<div class="flex animate-pulse gap-3">
							<span class="size-8 shrink-0 rounded-md bg-muted"></span>
							<div class="flex-1 space-y-2 py-0.5">
								<div class="h-3 w-2/3 rounded bg-muted"></div>
								<div class="h-2.5 w-1/3 rounded bg-muted"></div>
							</div>
						</div>
					{/each}
				</div>
			{:else if detail.context.hasCalendarEventsLoadFailed}
				<div class="flex items-center gap-3 rounded-lg bg-destructive/5 px-4 py-3 text-sm text-destructive">
					<CircleAlertIcon class="size-4 shrink-0" />
					{text.calendarEventsLoadFailed}
				</div>
			{:else if detail.context.calendarEvents.length}
				<div class="grid gap-2">
					{#each detail.context.calendarEvents as event (event.id)}
						<CalendarEventListCard
							cardTestID="team-status-calendar-event-card"
							buttonTestID="team-status-calendar-event"
							title={event.calendarEvent.title}
							color={event.calendarEvent.color}
							participants={calendarParticipantsFromUnknown(event.calendarEvent.participants)}
							start={new Date(event.calendarEvent.startISO)}
							isAllDay={event.calendarEvent.isAllDay}
							timeLabel={event.timeLabel}
							location={event.location}
							openEvent={() => openCalendarEvent(event.id)}
						/>
					{/each}
				</div>
			{:else}
				<Empty.Root class="min-h-24 border border-dashed bg-muted/10 p-4" data-testid="calendar-empty-state">
					<Empty.Header class="gap-1.5">
						<Empty.Media class="mb-0 text-muted-foreground">
							<CalendarDaysIcon class="size-5" aria-hidden="true" />
						</Empty.Media>
						<Empty.Title class="text-muted-foreground">{text.noCalendarEvents}</Empty.Title>
					</Empty.Header>
				</Empty.Root>
			{/if}
		</div>
	</section>

	<section class="px-5 py-5">
		<div class="flex items-center gap-2" data-testid="team-status-completed-work-header">
			<div class="flex items-center gap-2">
				<CheckCircle2Icon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" data-slot="completed-work-section-icon" />
				<h3 class="text-sm font-semibold">{text.completedWork}</h3>
			</div>
			{#if !detail.context.isCompletedWorkLoading && !detail.context.hasCompletedWorkLoadFailed}
				<span
					class={sectionCountBadgeClass}
					aria-label={`${text.completedWork} ${detail.context.completedTasks.length}`}
					data-slot="section-count-badge"
					data-testid="section-count-badge"
				>
					{detail.context.completedTasks.length}
				</span>
			{/if}
		</div>

		<div class="mt-4" data-testid="team-status-completed-task-list">
			{#if detail.context.isCompletedWorkLoading}
				<div class="grid gap-4" aria-label={text.loading}>
					{#each [0, 1] as loadingRow (loadingRow)}
						<div class="flex animate-pulse gap-3">
							<span class="size-8 shrink-0 rounded-md bg-muted"></span>
							<div class="flex-1 space-y-2 py-0.5">
								<div class="h-3 w-3/4 rounded bg-muted"></div>
								<div class="h-2.5 w-1/2 rounded bg-muted"></div>
							</div>
						</div>
					{/each}
				</div>
			{:else if detail.context.hasCompletedWorkLoadFailed}
				<div class="flex items-center gap-3 rounded-lg bg-destructive/5 px-4 py-3 text-sm text-destructive">
					<CircleAlertIcon class="size-4 shrink-0" />
					{text.completedWorkLoadFailed}
				</div>
			{:else if detail.context.completedTasks.length}
				<div class="grid gap-2">
					{#each detail.context.completedTasks as task (task.id)}
						<div data-testid="team-status-completed-task">
							<FlowTaskBoardCard
								task={task.task}
								businessFallback={flowBusinessFallback}
								openTask={openFlowTask}
								isDraggable={false}
							/>
						</div>
					{/each}
				</div>
			{:else}
				<Empty.Root class="min-h-24 border border-dashed bg-muted/10 p-4" data-testid="completed-work-empty-state">
					<Empty.Header class="gap-1.5">
						<Empty.Media class="mb-0 text-muted-foreground">
							<CheckCircle2Icon class="size-5" aria-hidden="true" />
						</Empty.Media>
						<Empty.Title class="text-muted-foreground">{text.noCompletedWork}</Empty.Title>
					</Empty.Header>
				</Empty.Root>
			{/if}
		</div>
	</section>
</div>


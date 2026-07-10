<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import CheckCircle2Icon from '@lucide/svelte/icons/circle-check-big';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import XIcon from '@lucide/svelte/icons/x';
	import { Textarea } from '$lib/components/ui/textarea';
	import CalendarEventContent from '../../calendar/embed/calendar-event-content.svelte';
	import FlowTaskBoardCard from '../../flow/flow-task-board-card.svelte';
	import { flowText } from '../../flow/text';
	import { getAttendanceState, type AttendanceEvent } from '../attendance-context.svelte';
	import { absencesForDate, absenceLabelText, hasAbsenceDetails } from '../shared/attendance-absence';
	import { absenceDisplayClass } from '../shared/color-tokens';
	import { localTimeMinutes } from '../shared/day-timeline';
	import DurationText from '../shared/duration-text.svelte';
	import WorkSegmentEditFields from '../shared/work-segment-edit-fields.svelte';
	import WorkSegmentSummary from '../shared/work-segment-summary.svelte';
	import type { AttendanceText } from '../text';
	import type { TeamStatusDayDetail } from './team-status-day-detail';

	type Props = {
		text: AttendanceText;
		detail: TeamStatusDayDetail;
	};

	type WorkEventDraft = {
		eventID: string;
		localDate: string;
		originalLocalTime: string;
		localTime: string;
		originalLocationID: string;
		locationID: string;
	};

	let { text, detail }: Props = $props();
	const attendance = getAttendanceState();
	let deletingAbsenceID = $state('');
	let absenceErrorID = $state('');
	let absenceErrorMessage = $state('');
	let isEditingWorkRecords = $state(false);
	let workEventDrafts = $state<Record<string, WorkEventDraft>>({});
	let workEditReason = $state('');
	let isSavingWorkRecords = $state(false);
	let workEditError = $state('');
	const sectionCountBadgeClass = 'inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-muted px-1.5 text-xs font-medium tabular-nums text-muted-foreground';

	const isOwnDay = $derived(detail.email === attendance.summary?.currentUserEmail);
	const ownDayAbsences = $derived(
		isOwnDay && attendance.summary
			? absencesForDate(attendance.summary.absences, detail.day.date, attendance.summary.currentUserEmail)
			: []
	);
	const attendanceLocations = $derived(attendance.summary?.locations ?? []);
	const flowBusinessFallback = $derived(
		text.dateLocale === 'ko-KR' ? flowText.ko.task.businessFallback : flowText.en.task.businessFallback
	);
	const canEditWorkRecords = $derived(
		isOwnDay && detail.day.segments.length > 0 && attendanceLocations.length > 0
	);
	const hasWorkRecordChanges = $derived(
		Object.values(workEventDrafts).some(
			(draft) => draft.localTime !== draft.originalLocalTime || draft.locationID !== draft.originalLocationID
		)
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

	function openWorkRecordEditor(): void {
		workEventDrafts = createWorkEventDrafts(attendance.summary?.events ?? []);
		workEditReason = '';
		workEditError = '';
		isEditingWorkRecords = true;
	}

	function closeWorkRecordEditor(): void {
		if (isSavingWorkRecords) return;
		resetWorkRecordEditor();
	}

	function resetWorkRecordEditor(): void {
		isEditingWorkRecords = false;
		workEventDrafts = {};
		workEditReason = '';
		workEditError = '';
	}

	function createWorkEventDrafts(events: AttendanceEvent[]): Record<string, WorkEventDraft> {
		const eventIDs = editableEventIDs();
		const fallbackLocationID = attendanceLocations[0]?.id ?? '';
		const drafts: Record<string, WorkEventDraft> = {};
		for (const event of events) {
			if (!eventIDs.has(event.id)) continue;
			const locationID = event.locationID || fallbackLocationID;
			drafts[event.id] = {
				eventID: event.id,
				localDate: event.localDate,
				originalLocalTime: shortTime(event.localTime),
				localTime: shortTime(event.localTime),
				originalLocationID: locationID,
				locationID
			};
		}
		return drafts;
	}

	function editableEventIDs(): Set<string> {
		const eventIDs = new Set<string>();
		for (const segment of detail.day.segments) {
			eventIDs.add(segment.startEventID);
			if (segment.endEventID) eventIDs.add(segment.endEventID);
		}
		return eventIDs;
	}

	function shortTime(localTime: string): string {
		return localTime.slice(0, 5);
	}

	function displayTimeForDraft(draft: WorkEventDraft | undefined, segmentDate: string, fallbackTime: string): string {
		if (draft?.localDate !== segmentDate) return fallbackTime;
		return draft.localTime;
	}

	function durationMinutesBetween(startTime: string, endTime: string): number {
		return Math.max(0, localTimeMinutes(endTime) - localTimeMinutes(startTime));
	}

	function updateEventTime(eventID: string | undefined, localTime: string): void {
		if (!eventID) return;
		const draft = workEventDrafts[eventID];
		if (draft) draft.localTime = localTime;
	}

	function updateSegmentLocation(segment: TeamStatusDayDetail['day']['segments'][number], locationID: string): void {
		const startDraft = workEventDrafts[segment.startEventID];
		if (startDraft) startDraft.locationID = locationID;
		if (segment.endReason !== 'clock_out' || !segment.endEventID) return;
		const endDraft = workEventDrafts[segment.endEventID];
		if (endDraft) endDraft.locationID = locationID;
	}

	async function saveWorkRecordChanges(): Promise<void> {
		if (!hasWorkRecordChanges || !workEditReason.trim() || isSavingWorkRecords) return;
		isSavingWorkRecords = true;
		workEditError = '';
		try {
			const updates = Object.values(workEventDrafts)
				.filter((draft) => draft.localTime !== draft.originalLocalTime || draft.locationID !== draft.originalLocationID)
				.map((draft) => ({
					eventID: draft.eventID,
					request: {
						localDate: draft.localDate,
						localTime: draft.localTime,
						locationID: draft.locationID,
						reason: workEditReason.trim()
					}
				}));
			await attendance.updateEvents(updates);
			resetWorkRecordEditor();
		} catch (error) {
			workEditError = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isSavingWorkRecords = false;
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
		<div class="flex items-center justify-between gap-4" data-testid="team-status-work-record-header">
			<div class="flex min-w-0 items-center gap-2">
				<Clock3Icon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
				<h3 class="truncate text-sm font-semibold">{text.workRecords}</h3>
				<span
					class={sectionCountBadgeClass}
					aria-label={text.locationSegmentCountTemplate.replace('{count}', String(detail.day.segments.length))}
					data-slot="section-count-badge"
					data-testid="section-count-badge"
				>
					{text.locationSegmentCountTemplate.replace('{count}', String(detail.day.segments.length))}
				</span>
			</div>
			<div class="flex shrink-0 items-center gap-2">
				{#if detail.day.durationMinutes !== undefined}
					<DurationText
						minutes={detail.day.durationMinutes}
						size="medium"
						tone={detail.day.tone === 'working' ? 'info' : 'default'}
					/>
				{/if}
				{#if canEditWorkRecords}
					<Button
						type="button"
						variant="ghost"
						size="icon-sm"
						aria-label={isEditingWorkRecords ? text.cancel : text.edit}
						disabled={isSavingWorkRecords}
						onclick={isEditingWorkRecords ? closeWorkRecordEditor : openWorkRecordEditor}
						data-testid="work-record-edit-button"
						data-state={isEditingWorkRecords ? 'editing' : 'idle'}
					>
						{#if isEditingWorkRecords}
							<XIcon class="size-4" />
						{:else}
							<PencilIcon class="size-4" />
						{/if}
					</Button>
				{/if}
			</div>
		</div>

		<div class="mt-4" data-testid="team-status-work-record-list">
			{#if detail.day.absenceDetail}
				<div class={`rounded-lg px-4 py-3 ${absenceDisplayClass(detail.day.absenceTone ?? 'leave')}`}>
					<div class="flex items-start justify-between gap-3">
						<div class="min-w-0">
							<p class="text-sm font-semibold">{detail.day.absenceDetail.label}</p>
							{#if detail.day.absenceDetail.reason}
								<p class="mt-1 text-sm opacity-80">{detail.day.absenceDetail.reason}</p>
							{/if}
							{#if detail.day.absenceDetail.createdBy}
								<p class="mt-1 text-xs opacity-70">
									{text.absenceCreatedByTemplate.replace('{user}', detail.day.absenceDetail.createdBy)}
								</p>
							{/if}
						</div>
						<span class="shrink-0 rounded-full bg-background/70 px-2.5 py-1 text-xs font-medium">
							{detail.day.absenceDetail.periodLabel}
						</span>
					</div>
				</div>
			{:else if detail.day.segments.length}
				<div class="grid gap-2">
					{#each detail.day.segments as segment (segment.id)}
						{@const startDraft = workEventDrafts[segment.startEventID]}
						{@const endDraft = segment.endEventID ? workEventDrafts[segment.endEventID] : undefined}
						{@const draftLocation = attendanceLocations.find((location) => location.id === startDraft?.locationID)}
						{@const displayStartTime = displayTimeForDraft(startDraft, detail.day.date, segment.startTime)}
						{@const displayEndTime = displayTimeForDraft(endDraft, detail.day.date, segment.endTime)}
						{@const displayDurationMinutes = startDraft
							? durationMinutesBetween(displayStartTime, displayEndTime)
							: segment.durationMinutes}
						<div
							class="rounded-lg border border-border/70 bg-card px-3.5 py-3 shadow-sm"
							data-testid="team-status-day-segment"
							data-state={segment.isOpen ? 'open' : 'closed'}
						>
							<WorkSegmentSummary
								locationName={draftLocation?.name ?? segment.locationName}
								locationColor={draftLocation?.color ?? segment.locationColor}
								startTime={displayStartTime}
								endTime={displayEndTime}
								durationMinutes={displayDurationMinutes}
								isOpen={segment.isOpen}
							/>
							{#if isEditingWorkRecords && startDraft}
								<WorkSegmentEditFields
									startTime={startDraft.localTime}
									endTime={endDraft?.localTime}
									locationID={startDraft.locationID}
									locations={attendanceLocations}
									isSaving={isSavingWorkRecords}
									{text}
									onStartTimeChange={(value) => updateEventTime(segment.startEventID, value)}
									onEndTimeChange={(value) => updateEventTime(segment.endEventID, value)}
									onLocationChange={(value) => updateSegmentLocation(segment, value)}
								/>
							{/if}
						</div>
					{/each}
				</div>
			{:else}
				<div class="flex items-center gap-3 rounded-lg bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
					<CircleAlertIcon class="size-4 shrink-0" />
					{text.eventNone}
				</div>
			{/if}
		</div>

		{#if isEditingWorkRecords}
			<div class="mt-4 grid gap-3 border-t pt-4" data-testid="work-record-edit-actions">
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					<span>{text.editReason}</span>
					<Textarea
						bind:value={workEditReason}
						placeholder={text.editReasonPlaceholder}
						disabled={isSavingWorkRecords}
						class="min-h-16 text-sm"
					/>
				</label>
				{#if workEditError}<p class="text-xs text-destructive">{workEditError}</p>{/if}
				<div class="flex justify-end gap-2">
					<Button type="button" variant="outline" size="sm" disabled={isSavingWorkRecords} onclick={closeWorkRecordEditor}>
						{text.cancel}
					</Button>
					<Button type="button" size="sm" disabled={!hasWorkRecordChanges || !workEditReason.trim() || isSavingWorkRecords} onclick={saveWorkRecordChanges}>
						{text.save}
					</Button>
				</div>
			</div>
		{/if}

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
						{@const calendarEvent = {
							title: event.calendarEvent.title,
							start: new Date(event.calendarEvent.startISO),
							allDay: event.calendarEvent.isAllDay
						}}
						<Card.Root
							size="sm"
							class="gap-0 rounded-md bg-info/10 py-0 text-info ring-info/20 transition-colors hover:bg-info/15 data-[size=sm]:gap-0 data-[size=sm]:py-0"
							data-testid="team-status-calendar-event-card"
						>
							<Card.Content class="p-0 group-data-[size=sm]/card:px-0">
								<button
									type="button"
									class="team-status-calendar-event min-h-11 w-full min-w-0 px-3 py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-info/40"
									aria-label={event.title}
									data-testid="team-status-calendar-event"
									onclick={() => openCalendarEvent(event.id)}
								>
									<CalendarEventContent
										event={calendarEvent}
										isAllDay={event.calendarEvent.isAllDay}
										timeLabel={event.timeLabel}
									/>
									{#if event.location}
										<span class="mt-1 flex min-w-0 items-center gap-1 text-xs text-info/70">
											<MapPinIcon class="size-3 shrink-0" aria-hidden="true" />
											<span class="truncate">{event.location}</span>
										</span>
									{/if}
								</button>
							</Card.Content>
						</Card.Root>
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

<style>
	:global(.team-status-calendar-event .calendar-event-content) {
		display: flex;
		min-width: 0;
		width: 100%;
		align-items: center;
		gap: 0.75rem;
	}

	:global(.team-status-calendar-event .calendar-event-title) {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		font-size: 0.875rem;
		font-weight: 600;
		line-height: 1.25rem;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	:global(.team-status-calendar-event .calendar-event-time) {
		flex: none;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 0.6875rem;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
</style>

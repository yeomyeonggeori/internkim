<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import XIcon from '@lucide/svelte/icons/x';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { startAttendanceMinuteClock } from '../shared/attendance-minute-clock';
	import { absenceDisplayClass } from '../shared/color-tokens';
	import DurationText from '../shared/duration-text.svelte';
	import WorkSegmentEditFields from '../shared/work-segment-edit-fields.svelte';
	import WorkSegmentSummary from '../shared/work-segment-summary.svelte';
	import type { AttendanceText } from '../text';
	import type { TeamStatusDayDetail } from './team-status-day-detail';
	import TeamStatusLeaveSegmentSummary from './team-status-leave-segment-summary.svelte';
	import { WorkRecordEditorState } from './work-record-editor.svelte';

	type Props = {
		text: AttendanceText;
		detail: TeamStatusDayDetail;
		sectionCountBadgeClass: string;
	};

	let { text, detail, sectionCountBadgeClass }: Props = $props();
	const attendance = getAttendanceState();
	const workRecordEditor = new WorkRecordEditorState({
		getSummary: () => attendance.summary,
		hasServerClock: () => attendance.serverClock !== null,
		getCurrentServerTime: () => attendance.currentServerTime(),
		updateEvents: (updates) => attendance.updateEvents(updates),
		get processingFailedMessage() {
			return text.processingFailed;
		}
	});
	const attendanceLocations = $derived(attendance.summary?.locations ?? []);
	const isOwnDay = $derived(detail.email === attendance.summary?.currentUserEmail);
	const canEditWorkRecords = $derived(
		isOwnDay && detail.day.segments.length > 0 && attendanceLocations.length > 0
	);
	const displayedSegmentCount = $derived(
		detail.day.timelineSegments.length || detail.day.segments.length
	);

	$effect(() => {
		if (!workRecordEditor.isEditing) return;
		if (!workRecordEditor.canUse || !workRecordEditor.matchesTimeZone(attendance.summary?.timeZone)) {
			workRecordEditor.reset();
			return;
		}
		return startAttendanceMinuteClock(
			(currentTime) => workRecordEditor.setCurrentTime(currentTime),
			() => attendance.currentServerTime()
		);
	});
</script>

<div class="flex items-center justify-between gap-4" data-testid="team-status-work-record-header">
	<div class="flex min-w-0 items-center gap-2">
		<Clock3Icon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
		<h3 class="truncate text-sm font-semibold">{text.workRecords}</h3>
		<span
			class={sectionCountBadgeClass}
			aria-label={text.locationSegmentCountTemplate.replace(
				'{count}',
				String(displayedSegmentCount)
			)}
			data-slot="section-count-badge"
			data-testid="section-count-badge"
		>
			{text.locationSegmentCountTemplate.replace('{count}', String(displayedSegmentCount))}
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
				aria-label={workRecordEditor.isEditing ? text.cancel : text.edit}
				disabled={workRecordEditor.isSaving || (!workRecordEditor.isEditing && !workRecordEditor.canUse)}
				onclick={workRecordEditor.isEditing
					? () => workRecordEditor.close()
					: () => workRecordEditor.open(detail.day.segments)}
				data-testid="work-record-edit-button"
				data-state={workRecordEditor.isEditing ? 'editing' : 'idle'}
			>
				{#if workRecordEditor.isEditing}
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
	{:else if detail.day.timelineSegments.length}
		<div class="grid gap-2">
			{#each detail.day.timelineSegments as timelineSegment (timelineSegment.id)}
				{#if timelineSegment.kind === 'leave'}
					<div
						class="rounded-lg border border-border/70 bg-card px-3.5 py-3 shadow-sm"
						data-testid="team-status-day-leave-segment"
					>
						<TeamStatusLeaveSegmentSummary
							label={timelineSegment.label}
							startTime={timelineSegment.startTime}
							endTime={timelineSegment.endTime}
							durationMinutes={timelineSegment.durationMinutes}
						/>
					</div>
				{:else}
					{@const segment = detail.day.segments.find(
						(candidate) => candidate.id === timelineSegment.id
					)}
					{#if segment}
						{@const startDraft = workRecordEditor.draftFor(segment.startEventID)}
						{@const endDraft = workRecordEditor.draftFor(segment.endEventID)}
						{@const draftLocation = attendanceLocations.find(
							(location) => location.id === startDraft?.locationID
						)}
						{@const displayStartTime = workRecordEditor.displayTime(
							startDraft,
							detail.day.date,
							segment.startTime
						)}
						{@const displayEndTime = workRecordEditor.displayTime(
							endDraft,
							detail.day.date,
							segment.endTime
						)}
						{@const displayDurationMinutes = startDraft
							? workRecordEditor.durationMinutes(displayStartTime, displayEndTime)
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
							{#if workRecordEditor.isEditing && startDraft}
								<WorkSegmentEditFields
									startTime={startDraft.localTime}
									endTime={endDraft?.localTime}
									locationID={startDraft.locationID}
									locations={attendanceLocations}
									isSaving={workRecordEditor.isSaving}
									startMaximumTime={workRecordEditor.maximumTimeFor(startDraft.localDate)}
									endMaximumTime={workRecordEditor.maximumTimeFor(endDraft?.localDate)}
									{text}
									onStartTimeChange={(value) =>
										workRecordEditor.updateEventTime(segment.startEventID, value)}
									onEndTimeChange={(value) =>
										workRecordEditor.updateEventTime(segment.endEventID, value)}
									onLocationChange={(value) =>
										workRecordEditor.updateSegmentLocation(segment, value)}
								/>
							{/if}
						</div>
					{/if}
				{/if}
			{/each}
		</div>
	{:else}
		<div class="flex items-center gap-3 rounded-lg bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
			<CircleAlertIcon class="size-4 shrink-0" />
			{text.eventNone}
		</div>
	{/if}
</div>

{#if workRecordEditor.isEditing}
	<div class="mt-4 grid gap-3 border-t pt-4" data-testid="work-record-edit-actions">
		<label class="grid gap-1 text-xs font-medium text-muted-foreground">
			<span>{text.editReason}</span>
			<Textarea
				bind:value={workRecordEditor.reason}
				placeholder={text.editReasonPlaceholder}
				disabled={workRecordEditor.isSaving}
				class="min-h-16 text-sm"
			/>
		</label>
		{#if workRecordEditor.errorMessage}<p class="text-xs text-destructive">{workRecordEditor.errorMessage}</p>{/if}
		<div class="flex justify-end gap-2">
			<Button type="button" variant="outline" size="sm" disabled={workRecordEditor.isSaving} onclick={() => workRecordEditor.close()}>
				{text.cancel}
			</Button>
			<Button type="button" size="sm" disabled={!workRecordEditor.canSave} onclick={() => workRecordEditor.save()}>
				{text.save}
			</Button>
		</div>
	</div>
{/if}

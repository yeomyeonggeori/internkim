<script lang="ts">
	import { absenceDisplayClass } from '../shared/color-tokens';
	import type { AttendanceText } from '../text';
	import { observeScrollOverflow } from './scroll-overflow-action';
	import type { TeamStatusDayDetail } from './team-status-day-detail';

	type Props = {
		text: AttendanceText;
		detail: TeamStatusDayDetail;
	};

	let { text, detail }: Props = $props();
	let hasWorkRecordScrollOverflow = $state(false);
	let hasCalendarEventScrollOverflow = $state(false);
	let hasCompletedTaskScrollOverflow = $state(false);

	const totalDurationLabel = $derived(detail.day.totalDurationLabel);
	const sectionFrameClass = 'relative min-h-0';
	const sectionListClass = 'grid gap-2 overflow-visible md:max-h-[10.5rem] md:overflow-y-auto md:pb-2 md:pr-1';
	const scrollFadeClass =
		'pointer-events-none absolute inset-x-0 bottom-0 hidden h-3 rounded-b-md bg-gradient-to-t from-popover/80 to-transparent md:block';
	const blockClass = 'rounded-md border border-border/70 bg-card px-3 py-2 shadow-sm';

	function setWorkRecordScrollOverflow(hasScrollOverflow: boolean) {
		hasWorkRecordScrollOverflow = hasScrollOverflow;
	}

	function setCalendarEventScrollOverflow(hasScrollOverflow: boolean) {
		hasCalendarEventScrollOverflow = hasScrollOverflow;
	}

	function setCompletedTaskScrollOverflow(hasScrollOverflow: boolean) {
		hasCompletedTaskScrollOverflow = hasScrollOverflow;
	}
</script>

<div class="grid gap-4 md:min-h-0 md:overflow-hidden" data-testid="team-status-day-detail-content">
	<div class="grid min-h-0 gap-2">
		<div class="flex items-center justify-between gap-3" data-testid="team-status-work-record-header">
			<div class="text-sm font-semibold">{text.workRecords}</div>
			{#if totalDurationLabel}
				<div class="shrink-0 text-sm font-semibold text-muted-foreground">{totalDurationLabel}</div>
			{/if}
		</div>
		<div class={sectionFrameClass}>
			<div class={sectionListClass} data-testid="team-status-work-record-list" use:observeScrollOverflow={setWorkRecordScrollOverflow}>
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
						<div class={`flex items-center justify-between gap-3 ${blockClass}`} data-testid="team-status-day-segment">
							<div class="min-w-0">
								<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
									{#if segment.locationName !== '-'}
										<span
											class={`size-2 shrink-0 rounded-full ${segment.locationColor ? '' : 'bg-success'}`}
											style:background-color={segment.locationColor}
										></span>
									{/if}
									<span class="min-w-0 truncate text-foreground">{segment.locationName}</span>
								</div>
								<div class="mt-0.5 text-xs tabular-nums text-muted-foreground">{segment.timeLabel}</div>
							</div>
							{#if segment.durationLabel}
								<div class={`shrink-0 text-sm font-medium ${segment.isOpen ? 'text-success' : 'text-muted-foreground'}`}>
									{segment.durationLabel}
								</div>
							{/if}
						</div>
					{/each}
				{:else}
					<div class="rounded-md border border-border/70 bg-muted/30 px-3 py-6 text-center text-sm text-muted-foreground shadow-sm">
						{text.eventNone}
					</div>
				{/if}
			</div>
			{#if hasWorkRecordScrollOverflow}
				<div class={scrollFadeClass} data-testid="team-status-section-scroll-fade"></div>
			{/if}
		</div>
	</div>

	<div class="grid min-h-0 gap-2">
		<div class="text-sm font-semibold">{text.calendarEvents}</div>
		<div class={sectionFrameClass}>
			<div class={sectionListClass} data-testid="team-status-calendar-event-list" use:observeScrollOverflow={setCalendarEventScrollOverflow}>
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
			{#if hasCalendarEventScrollOverflow}
				<div class={scrollFadeClass} data-testid="team-status-section-scroll-fade"></div>
			{/if}
		</div>
	</div>

	<div class="grid min-h-0 gap-2">
		<div class="text-sm font-semibold">{text.completedWork}</div>
		<div class={sectionFrameClass}>
			<div class={sectionListClass} data-testid="team-status-completed-task-list" use:observeScrollOverflow={setCompletedTaskScrollOverflow}>
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
			{#if hasCompletedTaskScrollOverflow}
				<div class={scrollFadeClass} data-testid="team-status-section-scroll-fade"></div>
			{/if}
		</div>
	</div>
</div>

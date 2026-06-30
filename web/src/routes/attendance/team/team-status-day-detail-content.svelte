<script lang="ts">
	import { absenceDisplayClass } from '../shared/color-tokens';
	import type { AttendanceText } from '../text';
	import type { TeamStatusDayDetail } from './team-status-day-detail';

	type Props = {
		text: AttendanceText;
		detail: TeamStatusDayDetail;
	};

	let { text, detail }: Props = $props();

	const totalDurationLabel = $derived(detail.day.totalDurationLabel);
</script>

<div class="grid gap-4" data-testid="team-status-day-detail-content">
	<div class="grid gap-2">
		<div class="text-sm font-semibold">{text.workRecords}</div>
		{#if detail.day.absenceDetail}
			<div class={`rounded-md border px-3 py-2 ${absenceDisplayClass(detail.day.absenceTone ?? 'leave')}`}>
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
			<div class="flex items-center justify-between gap-3">
				<div class="text-xs font-medium text-muted-foreground">{text.workSegments}</div>
				{#if totalDurationLabel}
					<div class="shrink-0 text-sm font-semibold text-muted-foreground">{totalDurationLabel}</div>
				{/if}
			</div>
			{#each detail.day.segments as segment (segment.id)}
				<div class="flex items-center justify-between gap-3 rounded-md border px-3 py-2" data-testid="team-status-day-segment">
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
			<div class="rounded-md border bg-muted/30 px-3 py-6 text-center text-sm text-muted-foreground">
				{text.eventNone}
			</div>
		{/if}
	</div>

	<div class="grid gap-2">
		<div class="text-sm font-semibold">{text.calendarEvents}</div>
		{#if detail.context.isCalendarEventsLoading}
			<div class="rounded-md border bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground">
				{text.loading}
			</div>
		{:else if detail.context.hasCalendarEventsLoadFailed}
			<div class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-4 text-center text-sm text-destructive">
				{text.calendarEventsLoadFailed}
			</div>
		{:else if detail.context.calendarEvents.length}
			{#each detail.context.calendarEvents as event (event.id)}
				<div class="rounded-md border px-3 py-2" data-testid="team-status-calendar-event">
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
			<div class="rounded-md border bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground">
				{text.noCalendarEvents}
			</div>
		{/if}
	</div>

	<div class="grid gap-2">
		<div class="text-sm font-semibold">{text.completedWork}</div>
		{#if detail.context.isCompletedWorkLoading}
			<div class="rounded-md border bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground">
				{text.loading}
			</div>
		{:else if detail.context.hasCompletedWorkLoadFailed}
			<div class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-4 text-center text-sm text-destructive">
				{text.completedWorkLoadFailed}
			</div>
		{:else if detail.context.completedTasks.length}
			{#each detail.context.completedTasks as task (task.id)}
				<div class="rounded-md border px-3 py-2" data-testid="team-status-completed-task">
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
			<div class="rounded-md border bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground">
				{text.noCompletedWork}
			</div>
		{/if}
	</div>
</div>

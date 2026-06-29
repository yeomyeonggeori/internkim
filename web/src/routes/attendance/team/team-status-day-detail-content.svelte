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
	{#if detail.day.absenceDetail}
		<div class="grid gap-2">
			<div class="text-sm font-semibold">{text.absenceDetails}</div>
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
		</div>
	{:else if detail.day.segments.length}
		<div class="grid gap-2">
			<div class="flex items-center justify-between gap-3">
				<div class="text-sm font-semibold">{text.workSegments}</div>
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
		</div>
	{:else}
		<div class="grid gap-2">
			<div class="text-sm font-semibold">{text.workSegments}</div>
			<div class="rounded-md border bg-muted/30 px-3 py-6 text-center text-sm text-muted-foreground">
				{text.eventNone}
			</div>
		</div>
	{/if}
</div>

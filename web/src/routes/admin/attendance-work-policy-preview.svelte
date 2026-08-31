<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import type { AdminPageText, AttendanceWorkPolicyRevision } from './admin-types';
	import { attendanceMonthlyTargetMinutes } from './attendance-work-policy-model';

	type Props = {
		revision: AttendanceWorkPolicyRevision;
		text: AdminPageText;
		currentMonth: string;
		holidayDates: string[];
	};

	let { revision, text, currentMonth, holidayDates }: Props = $props();

	function hours(minutes: number): string {
		if (minutes <= 0) return text.workSettings.noBaseline;
		const hourValue = Math.floor(minutes / 60);
		const minuteValue = minutes % 60;
		return minuteValue === 0
			? `${hourValue}${text.workSettings.hourUnit}`
			: `${hourValue}${text.workSettings.hourUnit} ${minuteValue}${text.workSettings.minuteUnit}`;
	}

	const modeLabel = $derived(text.workSettings[revision.workMode]);
	const monthlyTargetMinutes = $derived(
		attendanceMonthlyTargetMinutes(revision, currentMonth, holidayDates)
	);
</script>

<Card.Root class="h-fit @5xl:sticky @5xl:top-5">
	<Card.Header>
		<Card.Title>{text.workSettings.preview}</Card.Title>
		<Card.Description>{text.workSettings.previewDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<div class="grid gap-4 text-sm">
			<div class="flex items-center justify-between border-b pb-3">
				<span class="text-muted-foreground">{text.workSettings.workMode}</span>
				<strong>{modeLabel}</strong>
			</div>
			<div class="flex items-center justify-between border-b pb-3">
				<span class="text-muted-foreground">{text.workSettings.dailyTarget}</span>
				<strong>{revision.workMode === 'autonomous' ? text.workSettings.noBaseline : hours(revision.dailyTargetMinutes)}</strong>
			</div>
			<div class="flex items-center justify-between border-b pb-3">
				<span class="text-muted-foreground">{text.workSettings.weeklyTargetPreview}</span>
				<strong>{hours(revision.weeklyTargetMinutes)}</strong>
			</div>
			<div class="flex items-center justify-between border-b pb-3">
				<span class="text-muted-foreground">{text.workSettings.monthlyTarget}</span>
				<strong>{hours(monthlyTargetMinutes)}</strong>
			</div>
			{#if revision.workMode === 'flexible' && revision.coreTimeEnabled}
				<div class="flex items-center justify-between border-b pb-3">
					<span class="text-muted-foreground">{text.workSettings.coreTime}</span>
					<strong>{revision.coreStartTime}–{revision.coreEndTime}</strong>
				</div>
			{/if}
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{text.workSettings.nightHours}</span>
				<strong>{revision.nightStartTime}–{revision.nightEndTime}</strong>
			</div>
		</div>
	</Card.Content>
</Card.Root>

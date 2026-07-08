<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';
	import { getAttendanceState, type AttendanceAbsenceKind } from './attendance-context.svelte';
	import { addDays, todayDateInTimeZone } from './shared/attendance-date';
	import { attendanceText } from './text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	let kind = $state<AttendanceAbsenceKind>('leave');
	let startDate = $state('');
	let endDate = $state('');
	let reason = $state('');
	let isSubmitting = $state(false);
	let statusMessage = $state('');
	let errorMessage = $state('');
	let hasEditedDateRange = $state(false);
	let dateRangeDurationDays = $state(0);

	$effect(() => {
		const defaultDate = attendance.selectedDate || today;
		if (hasEditedDateRange) return;
		startDate = defaultDate;
		endDate = defaultDate;
		dateRangeDurationDays = 0;
	});

	const kindOptions = $derived([
		{ value: 'leave' as const, label: text.absenceKindLeave },
		{ value: 'other' as const, label: text.absenceKindOther }
	]);

	async function submitAbsence() {
		if (isSubmitting) return;
		isSubmitting = true;
		statusMessage = '';
		errorMessage = '';
		try {
			const createdAbsences = await attendance.createAbsence({
				kind,
				startDate,
				endDate,
				reason: reason.trim()
			});
			reason = '';
			hasEditedDateRange = false;
			statusMessage = createdAbsences.length === 0 ? text.absenceNoWeekdays : text.absenceCreated;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isSubmitting = false;
		}
	}

	function handleStartDateInput(event: Event) {
		const nextStartDate = event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : startDate;
		const shouldMoveEndDate = nextStartDate > startDate;
		startDate = nextStartDate;
		if (shouldMoveEndDate) {
			endDate = addDays(nextStartDate, dateRangeDurationDays);
		}
		hasEditedDateRange = true;
	}

	function handleEndDateInput(event: Event) {
		const nextEndDate = event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : endDate;
		endDate = nextEndDate;
		dateRangeDurationDays = dateDifferenceInDays(startDate, nextEndDate);
		hasEditedDateRange = true;
	}

	function dateDifferenceInDays(start: string, end: string): number {
		const startTime = Date.parse(`${start}T00:00:00Z`);
		const endTime = Date.parse(`${end}T00:00:00Z`);
		if (Number.isNaN(startTime) || Number.isNaN(endTime)) return 0;
		return Math.max(0, Math.round((endTime - startTime) / 86400000));
	}
</script>

<form
	class="grid gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		submitAbsence();
	}}
>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		<span>{text.absenceKind}</span>
		<Select.Root type="single" bind:value={kind} disabled={isSubmitting}>
			<Select.Trigger class="w-full">
				{kindOptions.find((option) => option.value === kind)?.label}
			</Select.Trigger>
			<Select.Content>
				{#each kindOptions as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		<span>{text.absenceStartDate}</span>
		<Input
			type="date"
			value={startDate}
			class="w-full min-w-0"
			disabled={isSubmitting}
			required
			oninput={handleStartDateInput}
		/>
	</label>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		<span>{text.absenceEndDate}</span>
		<Input
			type="date"
			value={endDate}
			min={startDate || undefined}
			class="w-full min-w-0"
			disabled={isSubmitting}
			required
			oninput={handleEndDateInput}
		/>
	</label>
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		<span>{text.absenceReason}</span>
		<Textarea
			class="min-h-16 resize-none text-sm"
			bind:value={reason}
			placeholder={text.absenceReasonPlaceholder}
			disabled={isSubmitting}
		/>
	</label>
	<div class="flex flex-col gap-2">
		<Button type="submit" class="w-full" disabled={isSubmitting || !startDate || !endDate}>
			{#if isSubmitting}
				<LoaderIcon class="size-3.5 animate-spin" />
			{/if}
			{text.createAbsence}
		</Button>
		{#if statusMessage}
			<p class="text-xs text-success">{statusMessage}</p>
		{/if}
		{#if errorMessage}
			<p class="text-xs text-destructive">{errorMessage}</p>
		{/if}
	</div>
</form>

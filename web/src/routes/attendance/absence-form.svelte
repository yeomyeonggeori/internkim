<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';
	import { getAttendanceState, type AttendanceAbsenceKind } from './attendance-context.svelte';
	import { todayDateInTimeZone } from './shared/attendance-date';
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

	$effect(() => {
		const defaultDate = attendance.selectedDate || today;
		if (hasEditedDateRange) return;
		startDate = defaultDate;
		endDate = defaultDate;
	});

	const kindOptions = $derived([
		{ value: 'leave' as const, label: text.absenceKindLeave },
		{ value: 'business_trip' as const, label: text.absenceKindBusinessTrip },
		{ value: 'day_off' as const, label: text.absenceKindDayOff },
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
</script>

<Card.Root>
	<Card.Header>
		<Card.Title class="text-base">{text.absenceFormTitle}</Card.Title>
	</Card.Header>
	<Card.Content>
		<form
			class="grid gap-3 sm:grid-cols-2"
			onsubmit={(event) => {
				event.preventDefault();
				submitAbsence();
			}}
		>
			<label class="grid gap-1 text-xs font-medium text-muted-foreground">
				<span>{text.absenceKind}</span>
				<select
					class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm text-foreground"
					bind:value={kind}
					disabled={isSubmitting}
				>
					{#each kindOptions as option (option.value)}
						<option value={option.value}>{option.label}</option>
					{/each}
				</select>
			</label>
			<label class="grid gap-1 text-xs font-medium text-muted-foreground">
				<span>{text.absenceStartDate}</span>
				<Input
					type="date"
					bind:value={startDate}
					max={endDate || undefined}
					disabled={isSubmitting}
					required
					oninput={() => {
						hasEditedDateRange = true;
					}}
				/>
			</label>
			<label class="grid gap-1 text-xs font-medium text-muted-foreground">
				<span>{text.absenceEndDate}</span>
				<Input
					type="date"
					bind:value={endDate}
					min={startDate || undefined}
					disabled={isSubmitting}
					required
					oninput={() => {
						hasEditedDateRange = true;
					}}
				/>
			</label>
			<label class="grid gap-1 text-xs font-medium text-muted-foreground sm:col-span-2">
				<span>{text.absenceReason}</span>
				<Textarea
					class="min-h-20 resize-none text-sm"
					bind:value={reason}
					placeholder={text.absenceReasonPlaceholder}
					disabled={isSubmitting}
				/>
			</label>
			<div class="flex flex-col gap-2 sm:col-span-2">
				<Button type="submit" class="w-full sm:w-fit" disabled={isSubmitting || !startDate || !endDate}>
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
	</Card.Content>
</Card.Root>

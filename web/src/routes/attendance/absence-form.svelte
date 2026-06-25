<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';
	import { getAttendanceState, type AttendanceAbsenceKind } from './attendance-context.svelte';
	import { addDays, todayDateInTimeZone } from './shared/attendance-date';
	import { attendanceText } from './text';

	type Props = {
		compact?: boolean;
	};

	let { compact = false }: Props = $props();
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

<Card.Root class={compact ? 'gap-1.5' : undefined}>
	<Card.Header class={compact ? 'pb-0' : undefined}>
		<Card.Title class={compact ? 'text-sm' : 'text-base'}>{text.absenceFormTitle}</Card.Title>
	</Card.Header>
	<Card.Content class={compact ? 'px-3 pb-3 pt-0' : undefined}>
		<form
			class={compact ? 'grid grid-cols-1 gap-2' : 'grid gap-3 sm:grid-cols-2'}
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
			<label class={compact ? 'grid gap-1 text-xs font-medium text-muted-foreground' : 'grid gap-1 text-xs font-medium text-muted-foreground sm:col-span-2'}>
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
			<label class={compact ? 'grid gap-1 text-xs font-medium text-muted-foreground' : 'grid gap-1 text-xs font-medium text-muted-foreground sm:col-span-2'}>
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
			<label class={compact ? 'grid gap-1 text-xs font-medium text-muted-foreground' : 'grid gap-1 text-xs font-medium text-muted-foreground sm:col-span-2'}>
				<span>{text.absenceReason}</span>
				<Textarea
					class={compact ? 'min-h-16 resize-none text-sm' : 'min-h-20 resize-none text-sm'}
					bind:value={reason}
					placeholder={text.absenceReasonPlaceholder}
					disabled={isSubmitting}
				/>
			</label>
			<div class={compact ? 'flex flex-col gap-2' : 'flex flex-col gap-2 sm:col-span-2'}>
				<Button type="submit" class={compact ? 'w-full' : 'w-full sm:w-fit'} disabled={isSubmitting || !startDate || !endDate}>
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

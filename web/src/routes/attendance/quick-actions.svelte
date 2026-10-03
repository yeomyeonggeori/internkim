<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { formatDisplayTime } from '$lib/components/time-text';
	import { cn } from '$lib/utils';
	import ZapIcon from '@lucide/svelte/icons/zap';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { getAttendanceState } from './attendance-context.svelte';
	import { type AttendanceWorkSegment } from './shared/attendance-aggregation';
	import { timeInTimeZone } from './shared/attendance-date';
	import { formatHoursMinutes } from './shared/attendance-format';
	import AttendanceProgressBar, { type AttendanceProgressSegment } from './shared/attendance-progress-bar.svelte';
	import { segmentWidthPercent } from './shared/day-timeline';
	import { elapsedWorkedMinutes } from './shared/attendance-day-events';
	import DurationText from './shared/duration-text.svelte';
	import LocationLabel from './shared/location-label.svelte';
	import { attendanceText } from './text';

	type SegmentBar = {
		id: string;
		widthPercent: number;
		color: string;
	};

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const todayDay = $derived(myAttendanceToday.day);
	const status = $derived(myAttendanceToday.status);
	const activeLeave = $derived(myAttendanceToday.activeLeave);
	const activeLeaveName = $derived(
		activeLeave
			? localizedLeaveTypeName(
					activeLeave.leaveTypeID,
					activeLeave.leaveTypeName,
					currentLocale.value
				)
			: ''
	);

	const elapsedMinutes = $derived(elapsedWorkedMinutes(todayDay, new Date()));

	const activeLocationName = $derived(
		todayDay.activeSegment?.locationName ?? todayDay.activeSegment?.locationID ?? text.location
	);

	const nextKind = $derived(myAttendanceToday.nextKind);
	const actionLabel = $derived(nextKind === 'clock_in' ? text.clockIn : text.clockOut);

	let selectedLocationID = $state<string>('');
	$effect(() => {
		if (!selectedLocationID && myAttendanceToday.defaultLocation) {
			selectedLocationID = myAttendanceToday.defaultLocation.id;
		}
	});

	let isToggling = $state(false);
	let errorMessage = $state('');
	let closeTime = $state('');
	let isCloseOpen = $state(false);
	const fieldID = $props.id();

	const stillOpen = $derived(myAttendanceToday.clockInNobodyClosed);

	async function handleCloseAndClockIn() {
		if (isToggling || !closeTime) return;
		isToggling = true;
		errorMessage = '';
		try {
			await myAttendanceToday.closeAndClockIn(closeTime, selectedLocationID);
			if (attendance.summary) await attendance.load();
			closeTime = '';
			isCloseOpen = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isToggling = false;
		}
	}

	async function handleToggle(confirmedEarlyReturn = false) {
		if (isToggling) return;
		isToggling = true;
		errorMessage = '';
		try {
			await myAttendanceToday.clock(
				nextKind,
				nextKind === 'clock_in' ? selectedLocationID : '',
				confirmedEarlyReturn
			);
			if (attendance.summary) await attendance.load();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isToggling = false;
		}
	}

	const locations = $derived(myAttendanceToday.locations);
	const todaySegmentBars = $derived(buildSegmentBars(todayDay.segments));
	const todayProgressSegments = $derived<AttendanceProgressSegment[]>(
		todaySegmentBars.map((segment) => ({
			id: segment.id,
			widthPercent: segment.widthPercent,
			color: segment.color
		}))
	);
	const showLocationPicker = $derived(nextKind === 'clock_in' && locations.length > 1);

	function buildSegmentBars(segments: AttendanceWorkSegment[]): SegmentBar[] {
		const currentTime = timeInTimeZone(myAttendanceToday.summary?.timeZone);
		return segments.map((segment) => {
			return {
				id: segment.id,
				widthPercent: segmentWidthPercent(segment, currentTime),
				color: segmentColor(segment) ?? 'var(--color-muted-foreground)',
			};
		});
	}

	function segmentColor(segment: AttendanceWorkSegment | undefined): string | undefined {
		if (!segment) return undefined;
		if (segment.locationID) {
			const locationByID = locations.find((location) => location.id === segment.locationID);
			if (locationByID?.color) return locationByID.color;
		}
		if (segment.locationName) {
			const locationByName = locations.find((location) => location.name === segment.locationName);
			if (locationByName?.color) return locationByName.color;
		}
		return undefined;
	}
</script>

<Card.Root class="gap-1.5">
	<Card.Header class="space-y-0 pb-0">
		<Card.Title class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
			<ZapIcon class="size-3.5" />
			{text.today}
		</Card.Title>
	</Card.Header>
	<Card.Content class="space-y-2.5 pt-0">
		{#if todayDay.clockIn}
			<div class="flex items-center justify-between gap-2">
				<DurationText minutes={elapsedMinutes} size="medium" tone="default" />
				{#if status === 'working'}
					<LocationLabel name={activeLocationName} class="min-w-0" />
				{/if}
			</div>
		{/if}
		{#if activeLeave}
			<div
				class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs"
				data-testid="active-leave-status"
			>
				<p class="font-medium text-destructive">{text.onLeave}</p>
				<p class="mt-0.5 text-muted-foreground">
					{activeLeaveName} · {activeLeave.startTime}–{activeLeave.endTime}
				</p>
			</div>
		{/if}
		{#if todaySegmentBars.length > 0}
			<span aria-hidden="true">
				<AttendanceProgressBar
					segments={todayProgressSegments}
					trackTestId="quick-actions-current-bar"
				/>
			</span>
		{/if}
		{#if todayDay.clockIn}
			<div class="flex min-w-0 items-center gap-3 text-xs tabular-nums text-muted-foreground">
				<span class="flex items-center gap-1">
					<LogInIcon class="size-3" />
					{formatDisplayTime(todayDay.clockIn.localTime)}
				</span>
				{#if todayDay.clockOut && status !== 'working'}
					<span class="flex items-center gap-1">
						<LogOutIcon class="size-3" />
						{formatDisplayTime(todayDay.clockOut.localTime)}
					</span>
				{/if}
			</div>
		{/if}

		{#if showLocationPicker}
			<Select.Root type="single" bind:value={selectedLocationID} disabled={isToggling}>
				<Select.Trigger size="sm" class="w-full text-xs">
					{locations.find((location) => location.id === selectedLocationID)?.name ?? text.location}
				</Select.Trigger>
				<Select.Content>
					{#each locations as location (location.id)}
						<Select.Item value={location.id} label={location.name}>{location.name}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}

		{#if nextKind === 'clock_out'}
			<AlertDialog.Root>
				<AlertDialog.Trigger class={cn(buttonVariants({ variant: 'outline' }), 'w-full')} disabled={isToggling}>
					{#if isToggling}
						<LoaderIcon class="size-3.5 animate-spin" />
					{:else}
						<LogOutIcon class="size-3.5" />
					{/if}
					{actionLabel}
				</AlertDialog.Trigger>
				<AlertDialog.Content>
					<AlertDialog.Header>
						<AlertDialog.Title>{text.clockOutConfirmTitle}</AlertDialog.Title>
						<AlertDialog.Description>
							{text.clockOutConfirmDescriptionTemplate.replace('{duration}', formatHoursMinutes(elapsedMinutes, text))}
						</AlertDialog.Description>
					</AlertDialog.Header>
					<AlertDialog.Footer>
						<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
						<AlertDialog.Action onclick={() => handleToggle()}>{text.clockOut}</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		{:else if activeLeave}
			<AlertDialog.Root>
				<AlertDialog.Trigger class={cn(buttonVariants(), 'w-full')} disabled={isToggling}>
					{#if isToggling}
						<LoaderIcon class="size-3.5 animate-spin" />
					{:else}
						<LogInIcon class="size-3.5" />
					{/if}
					{actionLabel}
				</AlertDialog.Trigger>
				<AlertDialog.Content>
					<AlertDialog.Header>
						<AlertDialog.Title>{text.earlyReturnConfirmTitle}</AlertDialog.Title>
						<AlertDialog.Description>
							{text.earlyReturnConfirmDescriptionTemplate.replace(
								'{endTime}',
								activeLeave.endTime
							)}
						</AlertDialog.Description>
					</AlertDialog.Header>
					<AlertDialog.Footer>
						<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
						<AlertDialog.Action onclick={() => handleToggle(true)}>
							{text.earlyReturnConfirmAction}
						</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		{:else if stillOpen}
			<AlertDialog.Root bind:open={isCloseOpen}>
				<AlertDialog.Trigger class={cn(buttonVariants(), 'w-full')} disabled={isToggling}>
					{#if isToggling}
						<LoaderIcon class="size-3.5 animate-spin" />
					{:else}
						<LogInIcon class="size-3.5" />
					{/if}
					{actionLabel}
				</AlertDialog.Trigger>
				<AlertDialog.Content>
					<AlertDialog.Header>
						<AlertDialog.Title>{text.clockOutNobodyRecordedTitle}</AlertDialog.Title>
						<AlertDialog.Description>
							{text.clockOutNobodyRecordedDescriptionTemplate
								.replace('{date}', stillOpen.localDate)
								.replace('{time}', formatDisplayTime(stillOpen.localTime))}
						</AlertDialog.Description>
					</AlertDialog.Header>
					<label class="grid gap-1.5 text-sm" for="{fieldID}-close">
						<span class="text-muted-foreground">{text.clockOutNobodyRecordedLabel}</span>
						<input
							id="{fieldID}-close"
							class="h-9 rounded-md border bg-background px-3 text-base"
							type="time"
							bind:value={closeTime}
							required
						/>
					</label>
					<AlertDialog.Footer>
						<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
						<AlertDialog.Action disabled={!closeTime} onclick={handleCloseAndClockIn}>
							{text.clockOutNobodyRecordedAction}
						</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		{:else}
			<Button class="w-full" onclick={() => handleToggle()} disabled={isToggling}>
				{#if isToggling}
					<LoaderIcon class="size-3.5 animate-spin" />
				{:else}
					<LogInIcon class="size-3.5" />
				{/if}
				{actionLabel}
			</Button>
		{/if}

		{#if errorMessage}
			<p class="text-xs text-destructive">{errorMessage}</p>
		{/if}
	</Card.Content>
</Card.Root>

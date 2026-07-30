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
	import { getAttendanceState, type AttendanceKind } from './attendance-context.svelte';
	import { computeDayEvents, statusForDay, type AttendanceWorkSegment } from './shared/attendance-aggregation';
	import { timeInTimeZone, todayDateInTimeZone } from './shared/attendance-date';
	import { formatHoursMinutes } from './shared/attendance-format';
	import { dayWidthPercent } from './shared/day-timeline';
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

	const today = $derived(todayDateInTimeZone(attendance.currentMonthSummary?.timeZone));

	const myEvents = $derived(
		(attendance.currentMonthSummary?.events ?? []).filter(
			(event) => event.email === attendance.currentMonthSummary?.currentUserEmail
		)
	);
	const myAbsences = $derived(
		(attendance.currentMonthSummary?.absences ?? []).filter(
			(absence) => absence.email === attendance.currentMonthSummary?.currentUserEmail
		)
	);

	const todayDay = $derived(computeDayEvents(today, myEvents, { currentDate: today }));
	const status = $derived(statusForDay(today, myEvents, myAbsences, today));
	const activeLeave = $derived(attendance.currentMonthSummary?.activeLeave);
	const activeLeaveName = $derived(
		activeLeave
			? localizedLeaveTypeName(
					activeLeave.leaveTypeID,
					activeLeave.leaveTypeName,
					currentLocale.value
				)
			: ''
	);

	const elapsedMinutes = $derived.by(() => {
		if (todayDay.activeSegment) {
			const elapsed = (Date.now() - new Date(todayDay.activeSegment.clockIn.occurredAt).getTime()) / 60000;
			return todayDay.workedMinutes + Math.max(0, Math.round(elapsed));
		}
		return todayDay.workedMinutes;
	});

	const activeLocationName = $derived(
		todayDay.activeSegment?.locationName ?? todayDay.activeSegment?.locationID ?? text.location
	);

	const nextKind = $derived<AttendanceKind>(
		activeLeave ? 'clock_in' : status === 'working' ? 'clock_out' : 'clock_in'
	);
	const actionLabel = $derived(nextKind === 'clock_in' ? text.clockIn : text.clockOut);

	let selectedLocationID = $state<string>('');
	$effect(() => {
		const locations = attendance.currentMonthSummary?.locations ?? [];
		if (!selectedLocationID) {
			const def = locations.find((l) => l.isDefault) ?? locations[0];
			if (def) selectedLocationID = def.id;
		}
	});

	let isToggling = $state(false);
	let errorMessage = $state('');

	async function handleToggle(confirmEarlyReturn = false) {
		if (isToggling) return;
		isToggling = true;
		errorMessage = '';
		try {
			await attendance.toggleAttendance(
				nextKind,
				nextKind === 'clock_in' ? selectedLocationID || undefined : undefined,
				confirmEarlyReturn
			);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isToggling = false;
		}
	}

	const locations = $derived(attendance.currentMonthSummary?.locations ?? []);
	const todaySegmentBars = $derived(buildSegmentBars(todayDay.segments));
	const showLocationPicker = $derived(nextKind === 'clock_in' && locations.length > 1);

	function buildSegmentBars(segments: AttendanceWorkSegment[]): SegmentBar[] {
		const currentTime = timeInTimeZone(attendance.currentMonthSummary?.timeZone);
		return segments.map((segment) => {
			return {
				id: segment.id,
				widthPercent: dayWidthPercent(segment.startTime, segment.isOpen ? currentTime : segment.endTime ?? segment.startTime),
				color: segmentColor(segment) ?? 'var(--color-muted-foreground)',
			};
		});
	}

	function segmentBarsTotalPercent(segmentBars: SegmentBar[]): number {
		const totalPercent = segmentBars.reduce((total, segment) => total + segment.widthPercent, 0);
		return Math.min(100, totalPercent);
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
				class="rounded-md border border-info/30 bg-info/5 px-3 py-2 text-xs"
				data-testid="active-leave-status"
			>
				<p class="font-medium text-info">{text.onLeave}</p>
				<p class="mt-0.5 text-muted-foreground">
					{activeLeaveName} · {activeLeave.startTime}–{activeLeave.endTime}
				</p>
			</div>
		{/if}
		{#if todaySegmentBars.length > 0}
			<div class="h-1.5 w-full rounded-full bg-muted" data-testid="quick-actions-current-bar" aria-hidden="true">
				<div class="flex h-full overflow-hidden rounded-full" style:width={`${segmentBarsTotalPercent(todaySegmentBars)}%`}>
					{#each todaySegmentBars as segment (segment.id)}
						<span class="h-full" style:flex-grow={segment.widthPercent} style:background-color={segment.color}></span>
					{/each}
				</div>
			</div>
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

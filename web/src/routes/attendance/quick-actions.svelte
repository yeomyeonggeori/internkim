<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ZapIcon from '@lucide/svelte/icons/zap';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';
	import { getAttendanceState, type AttendanceKind } from './attendance-context.svelte';
	import { computeDayEvents, statusForDay, type AttendanceWorkSegment } from './shared/attendance-aggregation';
	import { todayDateInTimeZone } from './shared/attendance-date';
	import { formatHoursMinutes } from './shared/attendance-format';
	import { attendanceText } from './text';

	type SegmentBar = {
		id: string;
		sharePercent: number;
		color: string;
		locationName: string;
		timeLabel: string;
		durationLabel: string;
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

	const elapsedMinutes = $derived.by(() => {
		if (todayDay.activeSegment) {
			const elapsed = (Date.now() - new Date(todayDay.activeSegment.clockIn.occurredAt).getTime()) / 60000;
			return todayDay.workedMinutes + Math.max(0, Math.round(elapsed));
		}
		return todayDay.workedMinutes;
	});

	const statusLabel = $derived(
		status === 'working'
			? text.working
			: status === 'finished'
				? text.finished
				: status === 'absence'
					? text.absence
					: text.absent
	);
	const statusDot = $derived(
		status === 'working'
			? 'bg-success'
			: status === 'absence'
				? 'bg-info'
				: status === 'finished'
					? 'bg-muted-foreground'
					: 'bg-muted-foreground/50'
	);

	const nextKind = $derived<AttendanceKind>(status === 'working' ? 'clock_out' : 'clock_in');
	const actionLabel = $derived(nextKind === 'clock_in' ? text.clockIn : text.clockOut);
	const currentStatusLabel = $derived(
		status === 'working'
			? todayDay.activeSegment?.locationName ?? todayDay.activeSegment?.locationID ?? statusLabel
			: statusLabel
	);

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

	async function handleToggle() {
		if (isToggling) return;
		isToggling = true;
		errorMessage = '';
		try {
			await attendance.toggleAttendance(
				nextKind,
				nextKind === 'clock_in' ? selectedLocationID || undefined : undefined
			);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isToggling = false;
		}
	}

	const locations = $derived(attendance.currentMonthSummary?.locations ?? []);
	const currentLocationColor = $derived(status === 'working' ? segmentColor(todayDay.activeSegment) : undefined);
	const todaySegmentBars = $derived(buildSegmentBars(todayDay.segments));
	const showLocationPicker = $derived(nextKind === 'clock_in' && locations.length > 1);

	function buildSegmentBars(segments: AttendanceWorkSegment[]): SegmentBar[] {
		if (segments.length === 0) return [];
		const segmentMinutes = segments.map((segment) => segmentDisplayMinutes(segment));
		const totalMinutes = segmentMinutes.reduce((total, minutes) => total + minutes, 0);
		const fallbackSharePercent = Math.round(100 / segments.length);
		return segments.map((segment, index) => ({
			id: segment.id,
			sharePercent: totalMinutes > 0 ? Math.max(1, Math.round((segmentMinutes[index] / totalMinutes) * 100)) : fallbackSharePercent,
			color: segmentColor(segment) ?? 'hsl(var(--muted-foreground))',
			locationName: segment.locationName ?? segment.locationID ?? text.location,
			timeLabel: segment.isOpen ? `${segment.startTime}~` : `${segment.startTime}-${segment.endTime ?? ''}`,
			durationLabel: formatHoursMinutes(segmentMinutes[index], text),
		}));
	}

	function segmentDisplayMinutes(segment: AttendanceWorkSegment): number {
		if (!segment.isOpen) return segment.workedMinutes;
		const elapsed = (Date.now() - new Date(segment.clockIn.occurredAt).getTime()) / 60000;
		return Math.max(0, Math.round(elapsed));
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
	<Card.Content class="space-y-2 pt-0">
		<div class="flex items-start justify-between gap-3">
			<div class="min-w-0 space-y-1.5">
				<div class="flex items-center gap-1.5">
					<span
						class={`size-2 rounded-full ${currentLocationColor ? '' : statusDot}`}
						style:background-color={currentLocationColor}
					></span>
					<span class="truncate text-sm font-semibold">{currentStatusLabel}</span>
				</div>
				{#if todayDay.clockIn}
					<div class="flex items-center gap-3 text-xs tabular-nums text-muted-foreground">
						<span class="flex items-center gap-1">
							<LogInIcon class="size-3" />
							{todayDay.clockIn.localTime}
						</span>
						{#if todayDay.clockOut && status !== 'working'}
							<span class="flex items-center gap-1">
								<LogOutIcon class="size-3" />
								{todayDay.clockOut.localTime}
							</span>
						{/if}
					</div>
				{/if}
			</div>
			{#if todaySegmentBars.length > 0}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class="-mt-0.5 grid w-24 shrink-0 grid-rows-[0.75rem_1rem] justify-items-end text-right focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
								data-testid="quick-actions-current-bar"
							>
								<div class="text-xs font-semibold leading-3 tabular-nums text-foreground">
									{formatHoursMinutes(elapsedMinutes, text)}
								</div>
								<div class="mt-1.5 flex h-1.5 w-full overflow-hidden rounded-full bg-muted" aria-hidden="true">
									{#each todaySegmentBars as segment (segment.id)}
										<span
											class="h-full min-w-1"
											style:width={`${segment.sharePercent}%`}
											style:background-color={segment.color}
										></span>
									{/each}
								</div>
							</button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content side="top" sideOffset={6} class="grid w-max max-w-[calc(100vw-2rem)] grid-cols-[0.375rem_max-content_max-content_max-content] gap-x-2 gap-y-1.5 overflow-x-auto">
						{#each todaySegmentBars as segment (segment.id)}
							<div class="contents text-left tabular-nums">
								<span
									class="size-1.5 shrink-0 rounded-full"
									style:background-color={segment.color}
								></span>
								<span class="whitespace-nowrap text-left">{segment.locationName}</span>
								<span class="whitespace-nowrap text-left">{segment.timeLabel}</span>
								<span class="whitespace-nowrap text-left">{segment.durationLabel}</span>
							</div>
						{/each}
					</Tooltip.Content>
				</Tooltip.Root>
			{/if}
		</div>

		{#if showLocationPicker}
			<select
				class="border-input bg-background h-8 w-full rounded-md border px-2 text-xs"
				bind:value={selectedLocationID}
				disabled={isToggling}
			>
				{#each locations as location (location.id)}
					<option value={location.id}>{location.name}</option>
				{/each}
			</select>
		{/if}

		<Button class="w-full" onclick={handleToggle} disabled={isToggling}>
			{#if isToggling}
				<LoaderIcon class="size-3.5 animate-spin" />
			{:else if nextKind === 'clock_in'}
				<LogInIcon class="size-3.5" />
			{:else}
				<LogOutIcon class="size-3.5" />
			{/if}
			{actionLabel}
		</Button>

		{#if errorMessage}
			<p class="text-xs text-destructive">{errorMessage}</p>
		{/if}
	</Card.Content>
</Card.Root>

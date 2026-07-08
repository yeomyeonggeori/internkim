<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Select from '$lib/components/ui/select';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { createPageText } from '$lib/i18n/page-text.svelte';
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
	import SegmentTooltip from './shared/segment-tooltip.svelte';
	import { attendanceText } from './text';

	type SegmentBar = {
		id: string;
		widthPercent: number;
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
	const activeLocationName = $derived(
		todayDay.activeSegment?.locationName ?? todayDay.activeSegment?.locationID ?? text.location
	);

	const nextKind = $derived<AttendanceKind>(status === 'working' ? 'clock_out' : 'clock_in');
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
	const todaySegmentBars = $derived(buildSegmentBars(todayDay.segments));
	const showLocationPicker = $derived(nextKind === 'clock_in' && locations.length > 1);

	function buildSegmentBars(segments: AttendanceWorkSegment[]): SegmentBar[] {
		const currentTime = timeInTimeZone(attendance.currentMonthSummary?.timeZone);
		return segments.map((segment) => {
			return {
				id: segment.id,
				widthPercent: dayWidthPercent(segment.startTime, segment.isOpen ? currentTime : segment.endTime ?? segment.startTime),
				color: segmentColor(segment) ?? 'hsl(var(--muted-foreground))',
				locationName: segment.locationName ?? segment.locationID ?? text.location,
				timeLabel: segment.isOpen ? `${segment.startTime}~` : `${segment.startTime}-${segment.endTime ?? ''}`,
				durationLabel: formatHoursMinutes(segmentDisplayMinutes(segment), text),
			};
		});
	}

	function segmentBarsTotalPercent(segmentBars: SegmentBar[]): number {
		const totalPercent = segmentBars.reduce((total, segment) => total + segment.widthPercent, 0);
		return Math.min(100, totalPercent);
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
	<Card.Content class="space-y-2.5 pt-0">
		<div class="flex items-center justify-between gap-2">
			<div class="flex min-w-0 items-center gap-1.5">
				<span class={`size-2 shrink-0 rounded-full ${statusDot}`}></span>
				<span class="shrink-0 text-sm font-semibold">{statusLabel}</span>
				{#if status === 'working'}
					<Badge variant="outline" class="min-w-0 shrink">{activeLocationName}</Badge>
				{/if}
			</div>
			{#if todayDay.clockIn}
				<span class="shrink-0 text-sm font-semibold tabular-nums">{formatHoursMinutes(elapsedMinutes, text)}</span>
			{/if}
		</div>
		{#if todaySegmentBars.length > 0}
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<button
							{...props}
							type="button"
							class="block w-full rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
							data-testid="quick-actions-current-bar"
						>
							<span class="block h-1.5 w-full rounded-full bg-muted" aria-hidden="true">
								<span class="flex h-full overflow-hidden rounded-full" style:width={`${segmentBarsTotalPercent(todaySegmentBars)}%`}>
									{#each todaySegmentBars as segment (segment.id)}
										<span class="h-full" style:flex-grow={segment.widthPercent} style:background-color={segment.color}></span>
									{/each}
								</span>
							</span>
						</button>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Content
					side="top"
					sideOffset={6}
					class="w-max max-w-[calc(100vw-2rem)] border bg-popover text-popover-foreground shadow-md"
					arrowClasses="hidden"
				>
					<SegmentTooltip rows={todaySegmentBars} />
				</Tooltip.Content>
			</Tooltip.Root>
		{/if}
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
						<AlertDialog.Action onclick={handleToggle}>{text.clockOut}</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		{:else}
			<Button class="w-full" onclick={handleToggle} disabled={isToggling}>
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

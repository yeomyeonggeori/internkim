<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import ZapIcon from '@lucide/svelte/icons/zap';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';
	import { getAttendanceState, type AttendanceKind } from './attendance-context.svelte';
	import { computeDayEvents, statusForDay } from './shared/attendance-aggregation';
	import { todayDateInTimeZone } from './shared/attendance-date';
	import { formatHoursMinutes } from './shared/attendance-format';

	const attendance = getAttendanceState();

	const today = $derived(todayDateInTimeZone(attendance.currentMonthSummary?.timeZone));

	const myEvents = $derived(
		(attendance.currentMonthSummary?.events ?? []).filter(
			(event) => event.email === attendance.currentMonthSummary?.currentUserEmail
		)
	);

	const todayMyEvents = $derived(myEvents.filter((event) => event.localDate === today));
	const todayDay = $derived(computeDayEvents(today, todayMyEvents));
	const status = $derived(statusForDay(today, myEvents));

	const elapsedMinutes = $derived.by(() => {
		if (todayDay.inProgress && todayDay.clockIn) {
			const elapsed = (Date.now() - new Date(todayDay.clockIn.occurredAt).getTime()) / 60000;
			return Math.max(0, Math.round(elapsed));
		}
		return todayDay.workedMinutes;
	});

	const statusLabel = $derived(
		status === 'working' ? '근무 중' : status === 'finished' ? '퇴근' : '미출근'
	);
	const statusDot = $derived(
		status === 'working' ? 'bg-success' : status === 'finished' ? 'bg-muted-foreground' : 'bg-muted-foreground/50'
	);

	const nextKind = $derived<AttendanceKind>(status === 'working' ? 'clock_out' : 'clock_in');
	const actionLabel = $derived(nextKind === 'clock_in' ? '출근' : '퇴근');

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
			errorMessage = error instanceof Error ? error.message : '처리 실패';
		} finally {
			isToggling = false;
		}
	}

	const locations = $derived(attendance.currentMonthSummary?.locations ?? []);
	const showLocationPicker = $derived(nextKind === 'clock_in' && locations.length > 1);
</script>

<Card.Root>
	<Card.Header class="space-y-0 pb-3">
		<Card.Title class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
			<ZapIcon class="size-3.5" />
			오늘
		</Card.Title>
	</Card.Header>
	<Card.Content class="space-y-3 pt-0">
		<div class="space-y-1.5">
			<div class="flex items-center gap-1.5">
				<span class={`size-2 rounded-full ${statusDot}`}></span>
				<span class="text-sm font-semibold">{statusLabel}</span>
			</div>
			{#if todayDay.clockIn}
				<div class="flex items-center gap-3 text-xs tabular-nums text-muted-foreground">
					<span class="flex items-center gap-1">
						<LogInIcon class="size-3" />
						{todayDay.clockIn.localTime}
					</span>
					{#if todayDay.clockOut}
						<span class="flex items-center gap-1">
							<LogOutIcon class="size-3" />
							{todayDay.clockOut.localTime}
						</span>
					{/if}
				</div>
				<p class="text-xs text-muted-foreground">{formatHoursMinutes(elapsedMinutes)}</p>
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

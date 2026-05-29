<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computeDayEvents, groupEventsByDay } from '../shared/attendance-aggregation';
	import { eachDayOfMonth, isWeekend, todayDateInTimeZone } from '../shared/attendance-date';

	const attendance = getAttendanceState();

	const targetEmail = $derived(attendance.selectedEmail || attendance.summary?.currentUserEmail || '');
	const personalEvents = $derived(
		attendance.summary ? attendance.summary.events.filter((e) => e.email === targetEmail) : []
	);
	const byDay = $derived(groupEventsByDay(personalEvents));
	const days = $derived(attendance.summary ? eachDayOfMonth(attendance.summary.month) : []);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const locationMap = $derived(
		new Map((attendance.summary?.locations ?? []).map((l) => [l.id, l]))
	);

	function leadingBlanks(firstDate: string): number[] {
		if (!firstDate) return [];
		const day = new Date(`${firstDate}T00:00:00Z`).getUTCDay();
		return Array.from({ length: day }, (_, i) => i);
	}

	function cellClass(date: string, hasClockIn: boolean): string {
		if (hasClockIn) return 'bg-emerald-100/70 dark:bg-emerald-950/40';
		if (isWeekend(date)) return 'bg-transparent text-muted-foreground';
		if (date < today) return 'bg-rose-100/70 dark:bg-rose-950/40';
		return 'bg-transparent';
	}

	function selectDate(date: string) {
		attendance.selectedDate = date;
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title class="text-base">{attendance.summary?.month ?? ''} 캘린더</Card.Title>
		{#if targetEmail}
			<p class="text-xs text-muted-foreground">{targetEmail}</p>
		{/if}
	</Card.Header>
	<Card.Content>
		<div class="grid grid-cols-7 gap-1.5 text-xs">
			{#each ['일', '월', '화', '수', '목', '금', '토'] as label}
				<div class="pb-1 text-center text-[11px] font-medium text-muted-foreground">{label}</div>
			{/each}
			{#each leadingBlanks(days[0] ?? '') as _}
				<div></div>
			{/each}
			{#each days as date (date)}
				{@const day = computeDayEvents(date, byDay.get(date) ?? [])}
				{@const location = day.clockIn?.locationID ? locationMap.get(day.clockIn.locationID) : null}
				<button
					type="button"
					class={`flex aspect-[1.05] flex-col justify-between rounded-md p-1.5 text-left transition ${cellClass(date, !!day.clockIn)} ${attendance.selectedDate === date ? 'outline outline-2 outline-foreground' : ''} ${date === today ? 'ring-1 ring-foreground/40' : ''}`}
					onclick={() => selectDate(date)}
				>
					<span class="text-sm font-semibold leading-none">{Number(date.slice(-2))}</span>
					{#if day.clockIn}
						<div class="flex flex-col gap-0.5 text-[11px] font-semibold leading-tight tabular-nums">
							<div class="flex items-center gap-1">
								<span class="text-muted-foreground">출</span>
								<span>{day.clockIn.localTime}</span>
							</div>
							<div class="flex items-center gap-1">
								<span class="text-muted-foreground">퇴</span>
								{#if day.inProgress}
									<span class="text-emerald-600 dark:text-emerald-400">진행중</span>
								{:else}
									<span>{day.clockOut?.localTime ?? '-'}</span>
								{/if}
							</div>
							{#if location}
								<span class="truncate text-[10px] font-medium text-muted-foreground">{location.name}</span>
							{/if}
						</div>
					{:else if date < today && !isWeekend(date)}
						<span class="text-[11px] font-semibold leading-tight text-rose-600 dark:text-rose-400">미출근</span>
					{/if}
				</button>
			{/each}
		</div>
	</Card.Content>
</Card.Root>

<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceMonthPicker from '../attendance-month-picker.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computeDayEvents, groupEventsByDay } from '../shared/attendance-aggregation';
	import { absenceLabelText, absencesForDate } from '../shared/attendance-absence';
	import { eachDayOfMonth, isWeekend, todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const targetEmail = $derived(attendance.summary?.currentUserEmail || '');
	const personalEvents = $derived(
		attendance.summary ? attendance.summary.events.filter((e) => e.email === targetEmail) : []
	);
	const personalAbsences = $derived(
		attendance.summary ? attendance.summary.absences.filter((absence) => absence.email === targetEmail) : []
	);
	const calendarMonth = $derived(attendance.summary?.month ?? attendance.selectedMonth);
	const byDay = $derived(groupEventsByDay(personalEvents));
	const days = $derived(attendance.summary ? eachDayOfMonth(attendance.summary.month) : []);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));

	function leadingBlanks(firstDate: string): number[] {
		if (!firstDate) return [];
		const day = new Date(`${firstDate}T00:00:00Z`).getUTCDay();
		return Array.from({ length: day }, (_, i) => i);
	}

	function cellClass(date: string, hasClockIn: boolean, hasAbsence: boolean): string {
		if (hasClockIn) return 'bg-emerald-100/70 dark:bg-emerald-950/40';
		if (hasAbsence) return 'bg-sky-100/70 dark:bg-sky-950/40';
		if (isWeekend(date)) return 'bg-transparent text-muted-foreground';
		if (date < today) return 'bg-rose-100/70 dark:bg-rose-950/40';
		return 'bg-transparent';
	}

	function selectDate(date: string) {
		attendance.selectedDate = date;
	}

	function selectMonth(month: string) {
		attendance.selectedMonth = month;
		attendance.selectedDate = '';
		attendance.load();
	}

	function calendarTime(localTime: string | undefined): string {
		return localTime ? localTime.slice(0, 5) : '-';
	}

	function segmentCountLabel(count: number): string {
		return text.locationSegmentCountTemplate.replace('{count}', String(count));
	}

	function weekdayLabels(): string[] {
		return [
			text.weekdaySunday,
			text.weekdayMonday,
			text.weekdayTuesday,
			text.weekdayWednesday,
			text.weekdayThursday,
			text.weekdayFriday,
			text.weekdaySaturday,
		];
	}
</script>

<Card.Root>
	<Card.Header class="flex min-w-0 flex-col items-stretch gap-3 pb-2 sm:flex-row sm:items-start sm:justify-between">
		<div class="min-w-0">
			<Card.Title class="min-w-0 truncate pt-1 text-base">
				{text.calendarTitleTemplate.replace('{month}', calendarMonth)}
			</Card.Title>
			{#if targetEmail}
				<p class="mt-1 truncate text-xs text-muted-foreground">{targetEmail}</p>
			{/if}
		</div>
		<div class="flex w-full shrink-0 justify-center sm:w-auto sm:justify-end">
			<AttendanceMonthPicker selectedMonth={calendarMonth} onSelectMonth={selectMonth} />
		</div>
	</Card.Header>
	<Card.Content>
		<div data-testid="personal-month-calendar-grid" class="grid min-w-0 grid-cols-7 gap-1 text-xs sm:gap-1.5">
			{#each weekdayLabels() as label}
				<div class="pb-1 text-center text-[11px] font-medium text-muted-foreground">{label}</div>
			{/each}
			{#each leadingBlanks(days[0] ?? '') as _}
				<div></div>
			{/each}
			{#each days as date (date)}
				{@const day = computeDayEvents(date, byDay.get(date) ?? [])}
				{@const dayAbsence = absencesForDate(personalAbsences, date, targetEmail)[0]}
				<button
					type="button"
					data-testid={`personal-calendar-day-${date}`}
					class={`flex min-h-[4.75rem] min-w-0 flex-col justify-between overflow-hidden rounded-md p-0.5 text-left transition sm:aspect-[1.05] sm:min-h-0 sm:p-1.5 ${cellClass(date, !!day.clockIn, !!dayAbsence)} ${attendance.selectedDate === date ? 'outline outline-2 outline-foreground' : ''} ${date === today ? 'ring-1 ring-foreground/40' : ''}`}
					onclick={() => selectDate(date)}
				>
					<span class="text-sm font-semibold leading-none">{Number(date.slice(-2))}</span>
					{#if day.clockIn}
						<div class="min-w-0 space-y-0.5 text-[9px] font-normal leading-tight tabular-nums sm:text-[11px] sm:font-semibold">
							<div
								data-testid="personal-calendar-time-line"
								class="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-0.5 sm:gap-x-1"
							>
								<span class="text-muted-foreground max-[420px]:hidden">{text.clockInShort}</span>
								<span class="min-w-0 truncate">{calendarTime(day.clockIn.localTime)}</span>
							</div>
							<div
								data-testid="personal-calendar-time-line"
								class="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-0.5 sm:gap-x-1"
							>
								<span class="text-muted-foreground max-[420px]:hidden">{text.clockOutShort}</span>
								{#if day.inProgress}
									<span class="min-w-0 truncate text-emerald-600 dark:text-emerald-400">{text.inProgress}</span>
								{:else}
									<span class="min-w-0 truncate">{calendarTime(day.clockOut?.localTime)}</span>
								{/if}
							</div>
							{#if day.segments.length > 1}
								<span class="mt-0.5 hidden w-fit max-w-full rounded bg-background/70 px-1 text-[10px] font-medium text-muted-foreground sm:inline-block">
									{segmentCountLabel(day.segments.length)}
								</span>
							{/if}
						</div>
					{:else if dayAbsence}
						<span class="text-[11px] font-semibold leading-tight text-info">
							{absenceLabelText(dayAbsence, text)}
						</span>
					{:else if date < today && !isWeekend(date)}
						<span class="text-[11px] font-semibold leading-tight text-rose-600 dark:text-rose-400">{text.absent}</span>
					{/if}
				</button>
			{/each}
		</div>
	</Card.Content>
</Card.Root>

<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computeHeatmap } from '../shared/attendance-aggregation';
	import { isWeekend, todayDateInTimeZone } from '../shared/attendance-date';
	import { HEAT_LEVEL_CLASSES } from '../shared/color-tokens';
	import { attendanceText } from '../text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const cells = $derived(
		attendance.summary ? computeHeatmap(attendance.summary.month, attendance.summary.events) : []
	);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));

	function leadingBlanks(firstDate: string): number[] {
		if (!firstDate) return [];
		const day = new Date(`${firstDate}T00:00:00Z`).getUTCDay();
		return Array.from({ length: day }, (_, i) => i);
	}

	function selectDate(date: string) {
		attendance.selectedDate = date;
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
	<Card.Header>
		<Card.Title class="text-base">{text.attendanceRateTitleTemplate.replace('{month}', attendance.summary?.month ?? '')}</Card.Title>
	</Card.Header>
	<Card.Content>
		<div class="grid grid-cols-7 gap-1 text-xs">
			{#each weekdayLabels() as label}
				<div class="text-center text-muted-foreground">{label}</div>
			{/each}
			{#each leadingBlanks(cells[0]?.date ?? '') as _}
				<div></div>
			{/each}
			{#each cells as cell (cell.date)}
				{@const weekend = isWeekend(cell.date)}
				<button
					type="button"
					class={`flex aspect-square flex-col justify-between rounded border border-border/40 p-1 text-left transition ${
						weekend && cell.presentCount === 0 ? 'bg-muted/40 text-muted-foreground' : HEAT_LEVEL_CLASSES[cell.level]
					} ${attendance.selectedDate === cell.date ? 'outline outline-2 outline-foreground' : ''} ${cell.date === today ? 'ring-2 ring-foreground/40' : ''}`}
					onclick={() => selectDate(cell.date)}
				>
					<div class="font-medium">{Number(cell.date.slice(-2))}</div>
					{#if cell.totalPeople > 0 && cell.presentCount > 0}
						<div class="text-[10px]">{cell.presentCount}/{cell.totalPeople}</div>
					{/if}
				</button>
			{/each}
		</div>
	</Card.Content>
</Card.Root>

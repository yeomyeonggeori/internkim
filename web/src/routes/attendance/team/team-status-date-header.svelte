<script lang="ts">
	import type { AttendanceText } from '../text';
	import { isWeekend } from '../shared/attendance-date';

	type Props = {
		date: string;
		today: string;
		text: AttendanceText;
	};

	let { date, today, text }: Props = $props();

	const header = $derived(dayHeaderParts(date));

	function dayHeaderParts(dateToFormat: string): { weekday: string; dateLabel: string } {
		const day = new Date(`${dateToFormat}T00:00:00Z`);
		return {
			weekday: weekdayLabel(day.getUTCDay()),
			dateLabel: `${day.getUTCMonth() + 1}/${day.getUTCDate()}`,
		};
	}

	function weekdayLabel(day: number): string {
		const labels = [
			text.weekdaySunday,
			text.weekdayMonday,
			text.weekdayTuesday,
			text.weekdayWednesday,
			text.weekdayThursday,
			text.weekdayFriday,
			text.weekdaySaturday,
		];
		return labels[day] ?? '';
	}

	function dayHeaderContainerClass(dateToStyle: string): string {
		if (dateToStyle === today) return 'z-[16] bg-foreground text-background';
		return 'z-10 bg-card';
	}

	function dayHeaderToneClass(dateToStyle: string): string {
		if (dateToStyle === today) return '';
		if (isWeekend(dateToStyle)) return 'text-destructive';
		return '';
	}
</script>

<div
	class={`sticky left-0 flex h-full items-center justify-center gap-1 whitespace-nowrap border-r p-1 text-xs ${dayHeaderContainerClass(date)}`}
	role="rowheader"
	data-testid={`team-status-day-${date}`}
>
	<span class={`font-medium tabular-nums ${dayHeaderToneClass(date)}`}>{header.dateLabel}</span>
	<span class={`font-semibold ${dayHeaderToneClass(date)}`}>{header.weekday}</span>
</div>

<script lang="ts">
	import type { AttendanceText } from '../text';
	import { isWeekend } from '../shared/attendance-date';

	type Props = {
		date: string;
		selectedDate: string;
		today: string;
		text: AttendanceText;
		onSelectDate: (date: string) => void;
	};

	let { date, selectedDate, today, text, onSelectDate }: Props = $props();

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

	function dayHeaderButtonClass(dateToStyle: string): string {
		if (dateToStyle === today) return 'border-primary bg-primary text-primary-foreground hover:bg-primary/90';
		if (selectedDate === dateToStyle) return 'border-primary bg-sky-50 text-foreground hover:bg-sky-50 dark:bg-sky-950/30';
		return 'border-transparent bg-transparent text-muted-foreground hover:bg-background';
	}

	function dayHeaderWeekdayClass(dateToStyle: string): string {
		if (dateToStyle === today) return 'text-primary-foreground';
		if (isWeekend(dateToStyle)) return 'text-destructive';
		return '';
	}
</script>

<div class="p-1 text-center" role="columnheader">
	<button
		type="button"
		class={`flex h-8 w-full flex-col items-center justify-center rounded-sm border text-xs transition ${dayHeaderButtonClass(date)}`}
		data-testid={`team-status-day-${date}`}
		onclick={() => onSelectDate(date)}
	>
		<span class={`font-semibold ${dayHeaderWeekdayClass(date)}`}>{header.weekday}</span>
		<span class="font-medium tabular-nums">{header.dateLabel}</span>
	</button>
</div>

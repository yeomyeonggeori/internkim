<!-- 캘린더 미니 월 날짜 셀 버튼을 렌더링합니다. -->
<script lang="ts">
	type Props = {
		date: Date;
		dateKey: string;
		dateLabel: string;
		hasEvent: boolean;
		isOtherMonth: boolean;
		isSelected: boolean;
		isToday: boolean;
		isWeekend: boolean;
		weekRange: 'start' | 'middle' | 'end' | '';
		onSelectDate: (date: Date) => void;
	};

	let { date, dateKey, dateLabel, hasEvent, isOtherMonth, isSelected, isToday, isWeekend, weekRange, onSelectDate }: Props = $props();
</script>

<button
	type="button"
	aria-label={dateLabel}
	aria-pressed={isSelected}
	data-mini-date-key={dateKey}
	data-selected={isSelected ? 'true' : 'false'}
	data-today={isToday ? 'true' : 'false'}
	data-weekend={isWeekend ? 'true' : 'false'}
	data-week-range={weekRange || undefined}
	class="relative mx-auto flex size-7 items-start justify-center rounded-none bg-transparent pt-0.5 text-[12px] tabular-nums transition-colors {isToday
		? 'font-bold text-primary ring-1 ring-primary/60'
		: isOtherMonth
			? isWeekend
				? 'text-red-400 opacity-45 hover:bg-accent/50'
				: 'text-muted-foreground opacity-45 hover:bg-accent/50'
			: isWeekend
				? 'text-red-400 hover:bg-accent/50'
				: 'text-foreground hover:bg-accent/50'}"
	onclick={() => onSelectDate(date)}
>
	<span class="mini-month-date-number">{date.getDate()}</span>
	<span
		class="mini-month-event-dot-slot"
		data-has-event={hasEvent ? 'true' : 'false'}
		data-other-month={isOtherMonth ? 'true' : 'false'}
		aria-hidden="true"
	></span>
</button>

<style>
	button[data-mini-date-key] {
		width: 100%;
		height: 1.75rem;
		min-height: 1.75rem;
		padding-top: 0;
	}

	button[data-mini-date-key][data-selected='true'] {
		box-shadow: inset 0 0 0 1px rgb(80 150 232 / 0.7);
	}

	button[data-mini-date-key][data-today='true'] {
		background: rgb(239 246 255);
	}

	button[data-mini-date-key][data-today='true'][data-selected='true'] {
		background: rgb(239 246 255);
	}

	button[data-mini-date-key] .mini-month-date-number {
		position: absolute;
		z-index: 1;
		top: 0.3125rem;
		left: 50%;
		isolation: isolate;
		line-height: 1;
		transform: translateX(-50%);
	}

	button[data-mini-date-key] .mini-month-event-dot-slot {
		position: absolute;
		z-index: 1;
		bottom: 0.25rem;
		left: 50%;
		width: 0.375rem;
		height: 0.375rem;
		border-radius: 9999px;
		background: rgb(59 130 246);
		opacity: 0;
		transform: translateX(-50%);
	}

	button[data-mini-date-key] .mini-month-event-dot-slot[data-has-event='true'] {
		opacity: 1;
	}

	button[data-mini-date-key] .mini-month-event-dot-slot[data-has-event='true'][data-other-month='true'] {
		opacity: 0.45;
	}

	button[data-mini-date-key][data-week-range] {
		z-index: 0;
		margin-right: 0;
		margin-left: 0;
		isolation: isolate;
		background: transparent !important;
		box-shadow: none;
		color: hsl(var(--foreground));
		font-weight: 400;
		line-height: 1;
		opacity: 1;
		transition-property: color;
	}

	button[data-mini-date-key][data-week-range]:hover,
	button[data-mini-date-key][data-week-range]:focus-visible {
		background: transparent !important;
	}

	button[data-mini-date-key][data-week-range][data-today='true'] {
		background: transparent !important;
	}

	button[data-mini-date-key][data-week-range][data-today='true'],
	button[data-mini-date-key][data-week-range][data-selected='true'] {
		z-index: 3;
	}

	button[data-mini-date-key][data-week-range]::before {
		position: absolute;
		z-index: -1;
		top: 0.125rem;
		right: 0;
		bottom: 0.125rem;
		left: 0;
		border-radius: 0;
		background: rgb(229 239 251);
		content: '';
	}

	button[data-mini-date-key][data-week-range][data-today='true'] .mini-month-date-number {
		color: rgb(29 78 216);
		font-weight: 700;
	}

	button[data-mini-date-key][data-week-range][data-weekend='true'] {
		color: rgb(248 113 113);
	}

	button[data-mini-date-key][data-week-range][data-selected='true'] {
		z-index: 3;
		border-radius: 0;
		outline: 1px solid rgb(80 150 232);
		outline-offset: -1px;
		box-shadow: none;
	}

	button[data-mini-date-key][data-week-range][data-today='true']:not([data-selected='true']) {
		z-index: 3;
		border-radius: 0;
		background: rgb(239 246 255) !important;
		box-shadow: none;
	}
</style>

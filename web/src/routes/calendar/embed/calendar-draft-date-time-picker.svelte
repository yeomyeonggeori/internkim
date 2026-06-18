<script lang="ts">
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import {
		draftDateTimeDateLabel,
		draftDateTimeHours,
		draftDateTimeMinutes,
		draftDateTimeMonthTitle,
		draftDateTimeMonthWeeks,
		monthDateFromDateKey,
		normalizedPickerTime,
		pickerTimeValue,
		shiftedMonth,
		type DraftDateTimePickerLocaleText,
		type DraftDateTimePickerKind,
		type DraftDateTimePickerValue
	} from './calendar-draft-date-time-format';
	import './calendar-draft-date-time-picker.css';

	type Props = {
		kind: DraftDateTimePickerKind;
		dateKey: string;
		time: string;
		allDay: boolean;
		label: string;
		localeCode: string;
		text: DraftDateTimePickerLocaleText;
		cancelText: string;
		save: (value: DraftDateTimePickerValue) => void;
		cancel: () => void;
	};

	let { kind, dateKey, time, allDay, label, localeCode, text, cancelText, save, cancel }: Props = $props();

	let selectedDateKey = $state('');
	let selectedHour = $state('00');
	let selectedMinute = $state('00');
	let visibleMonth = $state(monthDateFromDateKey('1970-01-01'));
	let activePickerKey = '';

	const hours = draftDateTimeHours();
	const minutes = draftDateTimeMinutes();
	const monthTitle = $derived(draftDateTimeMonthTitle(visibleMonth, localeCode));
	const monthWeeks = $derived(draftDateTimeMonthWeeks(visibleMonth));
	const weekDayLabels = $derived(
		Array.from({ length: 7 }, (_, dayIndex) =>
			new Date(2026, 5, 7 + dayIndex).toLocaleDateString(localeCode, { weekday: 'short' })
		)
	);

	$effect.pre(() => {
		const pickerKey = `${kind}:${dateKey}:${time}`;
		if (pickerKey === activePickerKey) return;
		activePickerKey = pickerKey;
		const normalizedTime = normalizedPickerTime(time);
		selectedDateKey = dateKey;
		selectedHour = normalizedTime.hour;
		selectedMinute = normalizedTime.minute;
		visibleMonth = monthDateFromDateKey(dateKey);
	});

	function selectDate(nextDateKey: string): void {
		selectedDateKey = nextDateKey;
		visibleMonth = monthDateFromDateKey(nextDateKey);
	}

	function saveSelection(): void {
		save({
			dateKey: selectedDateKey,
			time: pickerTimeValue(selectedHour, selectedMinute)
		});
	}
</script>

<div
	class="draft-date-time-picker"
	role="group"
	aria-label={label}
>
	<header class="draft-date-time-picker-header">
		<button
			type="button"
			class="draft-date-time-picker-nav"
			aria-label={text.previousMonth}
			onclick={() => {
				visibleMonth = shiftedMonth(visibleMonth, -1);
			}}
		>
			<ChevronLeftIcon class="size-4" />
		</button>
		<div class="draft-date-time-picker-title">
			<CalendarDaysIcon class="size-4" />
			<span>{monthTitle}</span>
		</div>
		<button
			type="button"
			class="draft-date-time-picker-nav"
			aria-label={text.nextMonth}
			onclick={() => {
				visibleMonth = shiftedMonth(visibleMonth, 1);
			}}
		>
			<ChevronRightIcon class="size-4" />
		</button>
	</header>

	<div class="draft-date-time-calendar" aria-label={text.selectDate}>
		{#each weekDayLabels as weekDayLabel}
			<span class="draft-date-time-weekday">{weekDayLabel}</span>
		{/each}
		{#each monthWeeks as week}
			{#each week as date}
				<button
					type="button"
					class="draft-date-time-day"
					class:draft-date-time-day-muted={!date.isCurrentMonth}
					class:draft-date-time-day-selected={date.dateKey === selectedDateKey}
					aria-label={draftDateTimeDateLabel(date.dateKey, localeCode)}
					aria-pressed={date.dateKey === selectedDateKey}
					onclick={() => selectDate(date.dateKey)}
				>
					{date.day}
				</button>
			{/each}
		{/each}
	</div>

	{#if !allDay}
		<div class="draft-date-time-time">
			<span class="draft-date-time-time-label">{text.timeRange}</span>
			<label class="draft-date-time-select-label">
				<select aria-label={text.hour} bind:value={selectedHour}>
					{#each hours as hour}
						<option value={hour}>{Number(hour)}</option>
					{/each}
				</select>
				<span>{text.hourSuffix}</span>
			</label>
			<label class="draft-date-time-select-label">
				<select aria-label={text.minute} bind:value={selectedMinute}>
					{#each minutes as minute}
						<option value={minute}>{Number(minute)}</option>
					{/each}
				</select>
				<span>{text.minuteSuffix}</span>
			</label>
		</div>
	{/if}

	<footer class="draft-date-time-picker-actions">
		<button type="button" class="draft-date-time-picker-cancel" onclick={cancel}>{cancelText}</button>
		<button type="button" class="draft-date-time-picker-save" onclick={saveSelection}>{text.save}</button>
	</footer>
</div>

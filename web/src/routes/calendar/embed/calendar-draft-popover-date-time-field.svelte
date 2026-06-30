<script lang="ts">
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import CalendarDraftDateTimePicker from './calendar-draft-date-time-picker.svelte';
	import {
		draftDateTimeKindLabel,
		draftDateTimePickerLabel,
		draftDateTimeSummary,
		type DraftDateTimePickerKind,
		type DraftDateTimePickerValue
	} from './calendar-draft-date-time-format';
	import {
		draftPopoverAllDayChanges,
		draftPopoverStartDateTimeChanges,
		type DraftPopoverState
	} from './calendar-draft-popover-state';
	import type { DraftPopoverText } from './calendar-draft-popover-text';
	import './calendar-draft-popover-date-time-field.css';

	type Props = {
		popover: DraftPopoverState;
		localeCode: string;
		text: DraftPopoverText;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
	};

	let { popover, localeCode, text, updatePopover }: Props = $props();

	let activeDateTimePicker = $state<DraftDateTimePickerKind | null>(null);
	let activePickerEventID = '';

	const startDateTimeSummary = $derived(draftDateTimeSummary(popover.dateKey, popover.startTime, popover.allDay));
	const endDateTimeSummary = $derived(draftDateTimeSummary(popover.endDateKey, popover.endTime, popover.allDay));
	const activePickerLabel = $derived(
		activeDateTimePicker ? draftDateTimePickerLabel(activeDateTimePicker, popover.allDay, text) : ''
	);

	function allDayChanges(event: Event): Partial<DraftPopoverState> {
		const allDay = event.currentTarget instanceof HTMLInputElement ? event.currentTarget.checked : false;
		return draftPopoverAllDayChanges(popover, allDay);
	}

	function dateTimeSummaryLabel(kind: DraftDateTimePickerKind): string {
		const summary = kind === 'start' ? startDateTimeSummary : endDateTimeSummary;
		return `${draftDateTimeKindLabel(kind, text)} ${summary}`;
	}

	function pickerDateKey(kind: DraftDateTimePickerKind): string {
		return kind === 'start' ? popover.dateKey : popover.endDateKey;
	}

	function pickerTime(kind: DraftDateTimePickerKind): string {
		return kind === 'start' ? popover.startTime : popover.endTime;
	}

	function saveDateTimePicker(value: DraftDateTimePickerValue): void {
		if (activeDateTimePicker === 'start') {
			updatePopover(draftPopoverStartDateTimeChanges(popover, value.dateKey, value.time));
			activeDateTimePicker = null;
			return;
		}
		if (activeDateTimePicker === 'end') {
			updatePopover({ endDateKey: value.dateKey, endTime: value.time });
			activeDateTimePicker = null;
		}
	}

	function toggleDateTimePicker(kind: DraftDateTimePickerKind): void {
		activeDateTimePicker = activeDateTimePicker === kind ? null : kind;
	}

	$effect(() => {
		if (popover.eventID === activePickerEventID) return;
		activePickerEventID = popover.eventID;
		activeDateTimePicker = null;
	});
</script>

<div class="draft-popover-field">
	<span class="draft-popover-field-label">{text.startDate}</span>
	<div class="draft-popover-datetime-inputs">
			<button
				type="button"
				class="draft-popover-date-time-summary"
				data-date-time-summary="start"
				aria-label={dateTimeSummaryLabel('start')}
				onclick={() => toggleDateTimePicker('start')}
			>
				<span class="draft-popover-date-time-label">{text.startDate}</span>
				<span class="draft-popover-date-time-value">{startDateTimeSummary}</span>
				<CalendarDaysIcon class="size-4" />
			</button>
			{#if activeDateTimePicker === 'start'}
				<CalendarDraftDateTimePicker
					kind="start"
					dateKey={pickerDateKey('start')}
					time={pickerTime('start')}
					allDay={popover.allDay}
					label={activePickerLabel}
					{localeCode}
					text={text.dateTimePicker}
					cancelText={text.cancel}
					save={saveDateTimePicker}
					cancel={() => {
						activeDateTimePicker = null;
					}}
				/>
			{/if}
			<button
				type="button"
				class="draft-popover-date-time-summary"
				data-date-time-summary="end"
				aria-label={dateTimeSummaryLabel('end')}
				onclick={() => toggleDateTimePicker('end')}
			>
				<span class="draft-popover-date-time-label">{text.endDate}</span>
				<span class="draft-popover-date-time-value">{endDateTimeSummary}</span>
				<CalendarDaysIcon class="size-4" />
			</button>
			{#if activeDateTimePicker === 'end'}
				<CalendarDraftDateTimePicker
					kind="end"
					dateKey={pickerDateKey('end')}
					time={pickerTime('end')}
					allDay={popover.allDay}
					label={activePickerLabel}
					{localeCode}
				text={text.dateTimePicker}
				cancelText={text.cancel}
				save={saveDateTimePicker}
				cancel={() => {
					activeDateTimePicker = null;
				}}
			/>
		{/if}
		<label class="draft-popover-all-day-toggle">
			<input
				type="checkbox"
				checked={popover.allDay}
				aria-label={text.allDay}
				onchange={(event) => updatePopover(allDayChanges(event))}
			/>
			<span>{text.allDay}</span>
		</label>
	</div>
</div>

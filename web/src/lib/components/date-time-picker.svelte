<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import DatePicker from './date-picker.svelte';
	import { datePickerText } from './date-picker-text';

	const midnight = '00:00';

	let {
		value = $bindable(),
		clearable = false,
		disabled = false,
		id,
		'aria-label': ariaLabel,
		timeLabel
	}: {
		value?: string;
		clearable?: boolean;
		disabled?: boolean;
		id?: string;
		'aria-label'?: string;
		timeLabel?: string;
	} = $props();

	const dateKey = $derived((value ?? '').slice(0, 10));
	const time = $derived((value ?? '').slice(11, 16));

	function changeDate(nextDateKey: string): void {
		value = nextDateKey ? `${nextDateKey}T${time || midnight}` : '';
	}

	function changeTime(nextTime: string): void {
		if (!dateKey || !nextTime) return;
		value = `${dateKey}T${nextTime}`;
	}
</script>

<div class="flex gap-2">
	<DatePicker
		{id}
		value={dateKey}
		onValueChange={changeDate}
		{clearable}
		{disabled}
		aria-label={ariaLabel}
		class="min-w-0 flex-1"
	/>
	<Input
		type="time"
		aria-label={timeLabel ?? [ariaLabel, datePickerText[currentLocale.value].time].filter(Boolean).join(' ')}
		class="bg-background w-28 appearance-none tabular-nums [&::-webkit-calendar-picker-indicator]:hidden [&::-webkit-calendar-picker-indicator]:appearance-none"
		value={time}
		disabled={disabled || !dateKey}
		onchange={(event) => changeTime(event.currentTarget.value)}
	/>
</div>

<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import { CalendarDate, getLocalTimeZone, type DateValue } from '@internationalized/date';
	import { Button } from '$lib/components/ui/button';
	import { Calendar } from '$lib/components/ui/calendar';
	import * as Popover from '$lib/components/ui/popover';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { cn } from '$lib/utils';
	import { datePickerText } from './date-picker-text';

	let {
		value = $bindable(),
		onValueChange,
		min,
		max,
		disabled = false,
		required = false,
		clearable = false,
		id,
		'aria-label': ariaLabel,
		'aria-invalid': ariaInvalid,
		class: className
	}: {
		value?: string;
		onValueChange?: (value: string) => void;
		min?: string;
		max?: string;
		disabled?: boolean;
		required?: boolean;
		clearable?: boolean;
		id?: string;
		'aria-label'?: string;
		'aria-invalid'?: boolean;
		class?: string;
	} = $props();

	let isOpen = $state(false);

	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const selectedDate = $derived(calendarDateFromDateKey(value ?? ''));
	const minimumDate = $derived(min ? calendarDateFromDateKey(min) : undefined);
	const maximumDate = $derived(max ? calendarDateFromDateKey(max) : undefined);
	const label = $derived(
		selectedDate
			? selectedDate.toDate(getLocalTimeZone()).toLocaleDateString(localeCode, {
					year: 'numeric',
					month: 'long',
					day: 'numeric'
				})
			: datePickerText[currentLocale.value].selectDate
	);

	function calendarDateFromDateKey(dateKey: string): CalendarDate | undefined {
		const [year, month, day] = dateKey.split('-').map(Number);
		if (!year || !month || !day) return undefined;
		return new CalendarDate(year, month, day);
	}

	function dateKeyFromCalendarDate(date: DateValue): string {
		return `${date.year}-${String(date.month).padStart(2, '0')}-${String(date.day).padStart(2, '0')}`;
	}

	function pickDate(date: DateValue | undefined): void {
		if (!date) return;
		value = dateKeyFromCalendarDate(date);
		onValueChange?.(value);
		isOpen = false;
	}

	function clearDate(): void {
		value = '';
		onValueChange?.(value);
		isOpen = false;
	}
</script>

<Popover.Root bind:open={isOpen}>
	<Popover.Trigger {disabled}>
		{#snippet child({ props })}
			<Button
				{...props}
				{id}
				variant="outline"
				aria-label={ariaLabel}
				aria-invalid={ariaInvalid}
				class={cn('w-full justify-between font-normal', !selectedDate && 'text-muted-foreground', className)}
			>
				{label}
				<ChevronDownIcon class="size-3.5 opacity-50" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-auto overflow-hidden p-0" align="start">
		<Calendar
			type="single"
			value={selectedDate}
			onValueChange={pickDate}
			minValue={minimumDate}
			maxValue={maximumDate}
			captionLayout="dropdown"
		/>
		{#if clearable && selectedDate}
			<div class="border-t p-2">
				<Button variant="ghost" size="sm" class="w-full" onclick={clearDate}>
					{datePickerText[currentLocale.value].clear}
				</Button>
			</div>
		{/if}
	</Popover.Content>
</Popover.Root>
{#if required}
	<input class="sr-only" tabindex="-1" aria-hidden="true" {value} {required} {disabled} />
{/if}

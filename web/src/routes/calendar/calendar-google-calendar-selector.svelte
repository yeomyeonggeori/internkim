<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import CheckIcon from '@lucide/svelte/icons/check';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import type { GoogleCalendarListEntry } from './calendar-layout-types';
	import type { CalendarGoogleAccountText } from './text';

	let {
		text,
		calendars,
		selectedCalendarID,
		isLoadingGoogleCalendars,
		isSelectingGoogleCalendar,
		selectionError,
		selectGoogleCalendar
	}: {
		text: CalendarGoogleAccountText;
		calendars: GoogleCalendarListEntry[];
		selectedCalendarID: string;
		isLoadingGoogleCalendars: boolean;
		isSelectingGoogleCalendar: boolean;
		selectionError: string;
		selectGoogleCalendar: (calendarID: string) => Promise<boolean>;
	} = $props();

	let selectedValue = $state('');
	let lastDefaultValue = '';

	$effect(() => {
		const defaultValue = selectedCalendarID || calendars[0]?.calendarID || '';
		if (defaultValue === lastDefaultValue) return;
		lastDefaultValue = defaultValue;
		selectedValue = defaultValue;
	});

	function selectCalendarValue(event: Event) {
		const target = event.currentTarget;
		selectedValue = target instanceof HTMLSelectElement ? target.value : '';
	}

	async function saveSelectedCalendar() {
		if (!selectedValue || isLoadingGoogleCalendars || isSelectingGoogleCalendar) return;
		await selectGoogleCalendar(selectedValue);
	}

	function calendarLabel(calendar: GoogleCalendarListEntry): string {
		if (calendar.primary) return `${calendar.summary} (${text.googleCalendarPrimaryLabel})`;
		return calendar.summary;
	}
</script>

<div class="grid gap-2 rounded-md border bg-muted/30 px-3 py-3">
	<div class="grid gap-1">
		<label for="google-calendar-select" class="text-xs font-medium text-muted-foreground">{text.googleCalendarSelectLabel}</label>
		{#if isLoadingGoogleCalendars}
			<p class="flex items-center gap-2 rounded-md border bg-background px-3 py-2 text-sm text-muted-foreground">
				<LoaderCircleIcon class="size-4 animate-spin" />
				<span>{text.googleCalendarListLoading}</span>
			</p>
		{:else if calendars.length === 0}
			<p class="rounded-md border bg-background px-3 py-2 text-sm text-muted-foreground">{text.googleCalendarListEmpty}</p>
		{:else}
			<select
				id="google-calendar-select"
				aria-label={text.googleCalendarSelectLabel}
				value={selectedValue}
				disabled={isSelectingGoogleCalendar}
				onchange={selectCalendarValue}
				class="h-9 w-full rounded-md border bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
			>
				{#each calendars as calendar}
					<option value={calendar.calendarID}>{calendarLabel(calendar)}</option>
				{/each}
			</select>
		{/if}
	</div>

	{#if selectionError}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">{selectionError}</p>
	{/if}

	<Button
		variant="outline"
		class="w-full justify-center gap-2"
		disabled={!selectedValue || isLoadingGoogleCalendars || isSelectingGoogleCalendar || calendars.length === 0}
		onclick={saveSelectedCalendar}
	>
		{#if isSelectingGoogleCalendar}
			<LoaderCircleIcon class="size-4 animate-spin" />
			<span>{text.googleCalendarSelectionSaving}</span>
		{:else}
			<CheckIcon class="size-4" />
			<span>{text.googleCalendarSelectionSaveAction}</span>
		{/if}
	</Button>
</div>

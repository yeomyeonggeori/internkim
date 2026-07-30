<script lang="ts">
	import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
	import type { CalendarLocaleText } from '../text';
	import CalendarDraftPopover from './calendar-draft-popover.svelte';
	import type { DraftPopoverState } from './calendar-draft-popover-state';
	import type { CalendarParticipant } from './calendar-participants';

	type CalendarOption = {
		id: string;
		name: string;
	};

	type Props = {
		calendarOptions: CalendarOption[];
		participantCandidates: CalendarParticipant[];
		cancelPopover: () => void;
		deletePopover: () => void;
		localeCode: string;
		popover: DraftPopoverState | null;
		savePopover: () => void;
		text: CalendarLocaleText;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
	};

	let {
		calendarOptions,
		participantCandidates,
		cancelPopover,
		deletePopover,
		localeCode,
		popover,
		savePopover,
		text,
		updatePopover
	}: Props = $props();

	let lastPopover: DraftPopoverState | null = null;
	const renderedPopover = $derived.by(() => {
		if (popover) lastPopover = popover;
		return lastPopover;
	});
	const popoverText = $derived({
		title: text.conflictField.title,
		titlePlaceholder: text.newEvent,
		allDay: text.allDay,
		location: text.conflictField.location,
		description: text.conflictField.description,
		participants: text.draftPopover.participants,
		participantsPlaceholder: text.draftPopover.participantsPlaceholder,
		participantsEmpty: text.draftPopover.participantsEmpty,
		participantsSummary: text.draftPopover.participantsSummary,
		calendar: text.draftPopover.calendar,
		cancel: text.draftPopover.cancel,
		complete: text.draftPopover.complete,
		delete: text.draftPopover.delete,
		startDate: text.draftPopover.startDate,
		endDate: text.draftPopover.endDate,
		startTime: text.draftPopover.startTime,
		endTime: text.draftPopover.endTime,
		dateTimePicker: text.draftPopover.dateTimePicker
	});
</script>

{#if popover && renderedPopover}
	<CalendarDraftPopover
		popover={renderedPopover}
		{localeCode}
		{calendarOptions}
		{participantCandidates}
		dialogLabel={renderedPopover.mode === 'edit' ? text.editEvent : text.newEvent}
		text={popoverText}
		{updatePopover}
		{savePopover}
		{cancelPopover}
		{deletePopover}
	/>
{/if}

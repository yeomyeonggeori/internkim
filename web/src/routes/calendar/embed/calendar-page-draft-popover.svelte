<script lang="ts">
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import type { CalendarLocaleText } from '../text';
	import { calendarAuditRows } from './calendar-audit';
	import CalendarDraftPopover from './calendar-draft-popover.svelte';
	import type { DraftPopoverState } from './calendar-draft-popover-state';
	import type { CalendarParticipant } from './calendar-participants';

	type CalendarOption = {
		id: string;
		name: string;
	};

	type Props = {
		auditEvent: Pick<DayFlowEvent, 'meta'> | null;
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
		auditEvent,
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
	const auditRows = $derived(
		calendarAuditRows(auditEvent, localeCode, {
			created: text.eventAuditCreated,
			updated: text.eventAuditUpdated
		})
	);
	const popoverText = $derived({
		title: text.conflictField.title,
		allDay: text.allDay,
		location: text.conflictField.location,
		description: text.conflictField.description,
		participants: text.draftPopover.participants,
		participantsPlaceholder: text.draftPopover.participantsPlaceholder,
		removeParticipantAction: text.draftPopover.removeParticipantAction,
		calendar: text.draftPopover.calendar,
		cancel: text.draftPopover.cancel,
		complete: text.draftPopover.complete,
		delete: text.draftPopover.delete,
		startDate: text.draftPopover.startDate,
		endDate: text.draftPopover.endDate,
		startTime: text.draftPopover.startTime,
		endTime: text.draftPopover.endTime,
		auditEmpty: text.draftPopover.auditEmpty,
		dateTimePicker: text.draftPopover.dateTimePicker
	});
</script>

{#if popover && renderedPopover}
	<CalendarDraftPopover
		popover={renderedPopover}
		{calendarOptions}
		{participantCandidates}
		{auditRows}
		auditLabel={text.eventAudit}
		dialogLabel={renderedPopover.mode === 'edit' ? text.editEvent : text.newEvent}
		text={popoverText}
		{updatePopover}
		{savePopover}
		{cancelPopover}
		{deletePopover}
	/>
{/if}

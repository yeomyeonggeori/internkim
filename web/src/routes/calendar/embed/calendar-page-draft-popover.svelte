<script lang="ts">
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import type { CalendarLocaleText } from '../text';
	import { calendarAuditRows } from './calendar-audit';
	import CalendarDraftPopover from './calendar-draft-popover.svelte';
	import type { DraftPopoverState } from './calendar-draft-popover-state';

	type CalendarOption = {
		id: string;
		name: string;
	};

	type Props = {
		auditEvent: Pick<DayFlowEvent, 'meta'> | null;
		calendarOptions: CalendarOption[];
		cancelPopover: () => void;
		deletePopover: () => void;
		isSaving: boolean;
		localeCode: string;
		popover: DraftPopoverState | null;
		repositionPopover: (size: { width: number; height: number }) => void;
		savePopover: () => void;
		text: CalendarLocaleText;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
	};

	let {
		auditEvent,
		calendarOptions,
		cancelPopover,
		deletePopover,
		isSaving,
		localeCode,
		popover,
		repositionPopover,
		savePopover,
		text,
		updatePopover
	}: Props = $props();

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

{#if popover}
	<CalendarDraftPopover
		{popover}
		{calendarOptions}
		{auditRows}
		auditLabel={text.eventAudit}
		{localeCode}
		{isSaving}
		text={popoverText}
		{updatePopover}
		{repositionPopover}
		{savePopover}
		{cancelPopover}
		{deletePopover}
	/>
{/if}

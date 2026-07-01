<script lang="ts">
	import { onDestroy } from 'svelte';
	import type { CalendarAuditRow } from './calendar-audit';
	import CalendarEventAuditCard from './calendar-event-audit-card.svelte';
	import CalendarDraftPopoverDateTimeField from './calendar-draft-popover-date-time-field.svelte';
	import type { DraftPopoverState } from './calendar-draft-popover-state';
	import type { DraftPopoverText } from './calendar-draft-popover-text';
	import CalendarParticipantSelector from './calendar-participant-selector.svelte';
	import type { CalendarParticipant } from './calendar-participants';

	type CalendarOption = {
		id: string;
		name: string;
	};

	type Props = {
		popover: DraftPopoverState;
		calendarOptions: CalendarOption[];
		participantCandidates: CalendarParticipant[];
		auditRows: CalendarAuditRow[];
		auditLabel: string;
		localeCode: string;
		text: DraftPopoverText;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
		setScrollState: (canScrollUp: boolean, canScrollDown: boolean) => void;
	};

	let { popover, calendarOptions, participantCandidates, auditRows, auditLabel, localeCode, text, updatePopover, setScrollState }: Props = $props();

	let bodyElement: HTMLElement | null = null;
	let scrollFrame: number | null = null;

	const auditEmptyText = $derived(popover.mode === 'edit' ? text.auditEmpty : '');

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement || event.currentTarget instanceof HTMLTextAreaElement
			? event.currentTarget.value
			: '';
	}

	function selectValue(event: Event): string {
		return event.currentTarget instanceof HTMLSelectElement ? event.currentTarget.value : '';
	}

	function observePopoverBody(element: HTMLElement): { destroy: () => void } {
		bodyElement = element;
		const resizeObserver = new ResizeObserver(schedulePopoverScrollState);
		resizeObserver.observe(element);
		schedulePopoverScrollState();
		return {
			destroy: () => {
				if (bodyElement === element) bodyElement = null;
				resizeObserver.disconnect();
				if (scrollFrame !== null) cancelAnimationFrame(scrollFrame);
			}
		};
	}

	function schedulePopoverScrollState(): void {
		if (scrollFrame !== null) cancelAnimationFrame(scrollFrame);
		scrollFrame = requestAnimationFrame(() => {
			scrollFrame = null;
			updatePopoverScrollState();
		});
	}

	function updatePopoverScrollState(): void {
		if (!bodyElement) {
			setScrollState(false, false);
			return;
		}
		const maxScrollTop = Math.max(0, bodyElement.scrollHeight - bodyElement.clientHeight);
		setScrollState(bodyElement.scrollTop > 1, bodyElement.scrollTop < maxScrollTop - 1);
	}

	$effect(() => {
		popover.title;
		popover.allDay;
		popover.description;
		popover.location;
		popover.participants;
		popover.calendarID;
		auditRows;
		schedulePopoverScrollState();
	});

	onDestroy(() => {
		if (scrollFrame !== null) cancelAnimationFrame(scrollFrame);
	});
</script>

<div class="draft-popover-body" use:observePopoverBody onscroll={updatePopoverScrollState}>
	<CalendarDraftPopoverDateTimeField {popover} {localeCode} {text} {updatePopover} />

	<label class="draft-popover-field">
		<span class="draft-popover-field-label">{text.location}</span>
		<input
			value={popover.location}
			aria-label={text.location}
			autocomplete="off"
			oninput={(event) => updatePopover({ location: inputValue(event) })}
		/>
	</label>

	<label class="draft-popover-field">
		<span class="draft-popover-field-label">{text.description}</span>
		<textarea
			value={popover.description}
			aria-label={text.description}
			rows="3"
			oninput={(event) => updatePopover({ description: inputValue(event) })}
		></textarea>
	</label>

	<div class="draft-popover-field">
		<span class="draft-popover-field-label">{text.participants}</span>
		<CalendarParticipantSelector
			participants={popover.participants}
			candidates={participantCandidates}
			label={text.participants}
			placeholder={text.participantsPlaceholder}
			removeLabel={text.removeParticipantAction}
			onChange={(participants) => updatePopover({ participants })}
		/>
	</div>

	<label class="draft-popover-field">
		<span class="draft-popover-field-label">{text.calendar}</span>
		<select
			value={popover.calendarID}
			aria-label={text.calendar}
			onchange={(event) => updatePopover({ calendarID: selectValue(event) })}
		>
			{#each calendarOptions as option (option.id)}
				<option value={option.id}>{option.name}</option>
			{/each}
		</select>
	</label>

	<CalendarEventAuditCard rows={auditRows} label={auditLabel} emptyText={auditEmptyText} />
</div>

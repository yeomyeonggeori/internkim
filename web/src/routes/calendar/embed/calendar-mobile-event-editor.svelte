<script lang="ts">
	import { getContext, tick } from 'svelte';
	import { type Event as DayFlowEvent, type MobileEventProps } from '@dayflow/core';
	import { calendarText } from '../text';
	import { isDraftEventID } from './calendar-draft-event-params';
	import { dateKey, draftPopoverAllDayChanges, type DraftPopoverState } from './calendar-draft-popover-state';
	import { eventEndDate, eventStartDate } from './calendar-event-mapping';
	import { calendarParticipantsFromUnknown, type CalendarParticipant } from './calendar-participants';
	import CalendarMobileEventEditorFields from './calendar-mobile-event-editor-fields.svelte';
	import {
		calendarMobileEditorStartDateTimeChanges,
		calendarMobileEditorUpdatedEvent
	} from './calendar-mobile-event-editor-state';
	import {
		mobileEventEditorLocaleContextKey,
		mobileEventEditorParticipantsContextKey,
		mobileEventEditorPersistenceContextKey,
		type MobileEventEditorLocaleContext,
		type MobileEventEditorParticipantsContext,
		type MobileEventEditorPersistenceContext
	} from './calendar-mobile-event-editor-types';
	import './calendar-mobile-event-editor.css';

	let { isOpen, onClose, onSave, onEventDelete, draftEvent, app }: MobileEventProps = $props();
	const persistence = getContext<MobileEventEditorPersistenceContext | undefined>(mobileEventEditorPersistenceContextKey);
	const localeContext = getContext<MobileEventEditorLocaleContext | undefined>(mobileEventEditorLocaleContextKey);
	const participantsContext = getContext<MobileEventEditorParticipantsContext | undefined>(
		mobileEventEditorParticipantsContextKey
	);

	let loadedEventKey = $state('');
	let title = $state('');
	let startDateKey = $state('');
	let endDateKey = $state('');
	let startTime = $state('');
	let endTime = $state('');
	let allDay = $state(false);
	let location = $state('');
	let description = $state('');
	let participants = $state<CalendarParticipant[]>([]);
	let calendarID = $state('');
	let isEditorOpen = $state(false);
	let titleInputElement: HTMLInputElement | undefined = $state();

	const text = $derived(localeContext?.getText() ?? calendarText.ko);
	const draftText = $derived(text.draftPopover);
	const calendars = $derived(app.getCalendars());
	const participantCandidates = $derived(participantsContext?.getCandidates() ?? []);
	const canEdit = $derived(draftEvent ? app.canMutateFromUI(draftEvent.id) : false);
	const isDraftEvent = $derived(Boolean(draftEvent && isDraftEventID(draftEvent.id)));
	const canDelete = $derived(Boolean(draftEvent && canEdit && onEventDelete));
	const editorTitle = $derived(isDraftEvent ? text.newEvent : text.editEvent);
	const canSave = $derived(Boolean(draftEvent && canEdit && isValidEventForm()));

	function loadDraftEvent(event: DayFlowEvent): void {
		const startDate = eventStartDate(event);
		const endDate = eventEndDate(event);
		title = event.title ?? '';
		startDateKey = dateKey(startDate);
		endDateKey = dateKey(endDate);
		startTime = timeValue(startDate);
		endTime = timeValue(endDate);
		allDay = event.allDay ?? false;
		location = typeof event.meta?.location === 'string' ? event.meta.location : '';
		description = event.description ?? '';
		participants = calendarParticipantsFromUnknown(event.meta?.participants);
		calendarID = event.calendarId ?? calendars[0]?.id ?? 'internkim';
	}

	function updateAllDay(checked: boolean): void {
		const changes = draftPopoverAllDayChanges(draftPopoverState(), checked);
		allDay = changes.allDay ?? allDay;
		startTime = changes.startTime ?? startTime;
		endTime = changes.endTime ?? endTime;
	}

	function handleEditorClick(event: MouseEvent): void {
		if (!isEditorOpen) return;
		const action = editorAction(event.target);
		if (action) {
			event.preventDefault();
			if (action === 'close') closeEditor();
			if (action === 'save') void saveEvent();
			if (action === 'delete') deleteEvent();
			return;
		}
		if (toggleAllDayFromSwitchClick(event)) return;
		openPickerFromFieldClick(event);
	}

	function handleEditorFieldEvent(event: Event): void {
		if (!isEditorOpen) return;
		const target = event.target;
		if (!(target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement)) {
			return;
		}
		const field = target.dataset.mobileEditorField;
		if (!field) return;
		if (field === 'title') title = target.value;
		if (field === 'startDate') updateStartDateTime({ startDateKey: target.value });
		if (field === 'startTime') updateStartDateTime({ startTime: target.value });
		if (field === 'endDate') endDateKey = target.value;
		if (field === 'endTime') endTime = target.value;
		if (field === 'allDay' && target instanceof HTMLInputElement) updateAllDay(target.checked);
		if (field === 'location') location = target.value;
		if (field === 'description') description = target.value;
		if (field === 'calendar') calendarID = target.value;
	}

	function updateStartDateTime(changes: { startDateKey?: string; startTime?: string }): void {
		const fields = calendarMobileEditorStartDateTimeChanges(
			{
				startDateKey,
				endDateKey,
				startTime,
				endTime,
				allDay
			},
			changes
		);
		startDateKey = fields.startDateKey;
		endDateKey = fields.endDateKey;
		startTime = fields.startTime;
		endTime = fields.endTime;
	}

	function handleEditorKeydown(event: KeyboardEvent): void {
		if (!isEditorOpen) return;
		const target = event.target;
		if (!(target instanceof HTMLInputElement)) return;
		if (target.dataset.mobileEditorField !== 'title') return;
		if (event.key === 'Enter') void saveEvent();
		if (event.key === 'Escape') closeEditor();
	}

	function editorAction(target: EventTarget | null): 'close' | 'save' | 'delete' | null {
		if (!(target instanceof Element)) return null;
		const actionElement = target.closest<HTMLElement>('[data-mobile-editor-action]');
		if (!actionElement) return null;
		if (!actionElement.closest('.calendar-mobile-event-editor')) return null;
		const action = actionElement.dataset.mobileEditorAction;
		if (action === 'close' || action === 'save' || action === 'delete') return action;
		return null;
	}

	function toggleAllDayFromSwitchClick(event: MouseEvent): boolean {
		if (!canEdit) return false;
		if (!(event.target instanceof Element)) return false;
		const switchField = event.target.closest<HTMLElement>('[data-mobile-editor-switch-field]');
		if (!switchField || !switchField.closest('.calendar-mobile-event-editor')) return false;
		if (event.target instanceof HTMLInputElement) return false;
		event.preventDefault();
		updateAllDay(!allDay);
		return true;
	}

	function openPickerFromFieldClick(event: MouseEvent): void {
		if (!(event.target instanceof Element)) return;
		const pickerField = event.target.closest<HTMLElement>('[data-mobile-editor-picker-field]');
		if (!pickerField || !pickerField.closest('.calendar-mobile-event-editor')) return;
		const clickedInput =
			event.target instanceof HTMLInputElement && event.target.dataset.mobileEditorField ? event.target : null;
		const input = clickedInput ?? pickerField.querySelector<HTMLInputElement>('input[data-mobile-editor-field]');
		if (!input || input.disabled) return;
		input.focus({ preventScroll: true });
		if (input.type !== 'date' && input.type !== 'time') return;
		if (typeof input.showPicker !== 'function') {
			input.click();
			return;
		}
		try {
			input.showPicker();
		} catch (error: unknown) {
			if (!(error instanceof DOMException)) throw error;
			input.click();
		}
	}

	async function saveEvent(): Promise<void> {
		if (!draftEvent || !canSave) return;
		const updatedEvent = calendarMobileEditorUpdatedEvent({
			draftEvent,
			title,
			description,
			start: startDate(),
			end: endDate(),
			allDay,
			calendarID,
			location,
			participants
		});
		if (persistence) {
			isEditorOpen = false;
			onClose();
			await persistence.saveEvent(updatedEvent);
			return;
		}
		onSave(updatedEvent);
	}

	function closeEditor(): void {
		isEditorOpen = false;
		onClose();
	}

	function deleteEvent(): void {
		if (!draftEvent || !onEventDelete) return;
		isEditorOpen = false;
		onEventDelete(draftEvent.id);
	}

	function draftPopoverState(): DraftPopoverState {
		return {
			mode: isDraftEvent ? 'create' : 'edit',
			eventID: draftEvent?.id ?? '',
			title,
			dateKey: startDateKey,
			endDateKey,
			startTime,
			endTime,
			allDay,
			location,
			description,
			participants,
			calendarID,
			anchor: null,
			position: {
				left: 0,
				top: 0,
				width: 0,
				arrowTop: 0,
				arrowSide: 'left',
				isReady: false
			}
		};
	}

	function isValidEventForm(): boolean {
		if (!title.trim()) return false;
		const start = startDate().getTime();
		const end = endDate().getTime();
		return allDay ? end >= start : end > start;
	}

	function startDate(): Date {
		return dateFromDateKeyAndTime(startDateKey, allDay ? '00:00' : startTime);
	}

	function endDate(): Date {
		return dateFromDateKeyAndTime(endDateKey, allDay ? '00:00' : endTime);
	}

	function dateFromDateKeyAndTime(selectedDateKey: string, selectedTime: string): Date {
		const [year = '1970', month = '1', day = '1'] = selectedDateKey.split('-');
		const [hour = '0', minute = '0'] = selectedTime.split(':');
		return new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), 0, 0);
	}

	function timeValue(date: Date): string {
		return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
	}

	$effect(() => {
		if (!isOpen || !draftEvent) {
			loadedEventKey = '';
			isEditorOpen = false;
			return;
		}
		const nextEventKey = `${draftEvent.id}:${String(draftEvent.start)}:${String(draftEvent.end)}`;
		if (loadedEventKey === nextEventKey) return;
		loadedEventKey = nextEventKey;
		isEditorOpen = true;
		loadDraftEvent(draftEvent);
	});

	$effect(() => {
		if (calendarID || calendars.length === 0) return;
		calendarID = calendars[0].id;
	});

	$effect(() => {
		if (!isEditorOpen || typeof document === 'undefined') return;
		document.addEventListener('click', handleEditorClick, true);
		document.addEventListener('input', handleEditorFieldEvent, true);
		document.addEventListener('change', handleEditorFieldEvent, true);
		document.addEventListener('keydown', handleEditorKeydown, true);
		document.body.style.overflow = 'hidden';
		document.documentElement.style.overflow = 'hidden';
		return () => {
			document.removeEventListener('click', handleEditorClick, true);
			document.removeEventListener('input', handleEditorFieldEvent, true);
			document.removeEventListener('change', handleEditorFieldEvent, true);
			document.removeEventListener('keydown', handleEditorKeydown, true);
			document.body.style.overflow = '';
			document.documentElement.style.overflow = '';
		};
	});

	$effect(() => {
		if (!isEditorOpen || !isDraftEvent || !canEdit || !titleInputElement) return;
		void tick().then(() => {
			titleInputElement?.focus({ preventScroll: true });
			titleInputElement?.select();
		});
	});
</script>

{#if isEditorOpen && draftEvent}
	<div class="df-portal df-mobile-event-drawer calendar-mobile-event-editor">
		<button
			type="button"
			class="df-mobile-event-drawer-backdrop"
			aria-label={draftText.cancel}
			data-mobile-editor-action="close"
		></button>
		<section class="df-mobile-event-drawer-panel df-animate-slide-up">
			<header class="df-mobile-event-drawer-header">
				<button type="button" class="df-mobile-event-drawer-header-action" data-mobile-editor-action="close">
					{draftText.cancel}
				</button>
				<span class="df-mobile-event-drawer-title">{editorTitle}</span>
				{#if canEdit}
					<button
						type="button"
						class="df-mobile-event-drawer-header-action df-mobile-event-drawer-header-action-primary"
						class:df-mobile-event-drawer-header-action-disabled={!canSave}
						disabled={!canSave}
						data-mobile-editor-action="save"
					>
						{draftText.complete}
					</button>
				{:else}
					<span class="df-mobile-event-drawer-header-spacer"></span>
				{/if}
			</header>

			<CalendarMobileEventEditorFields
				{text}
				{draftText}
				{calendars}
				{canEdit}
				{canDelete}
				{title}
				{startDateKey}
				{endDateKey}
				{startTime}
				{endTime}
				{allDay}
				{location}
				{description}
				bind:participants
				{participantCandidates}
				{calendarID}
				onParticipantsChange={(nextParticipants) => {
					participants = nextParticipants;
				}}
				bind:titleInputElement
			/>
		</section>
	</div>
{/if}

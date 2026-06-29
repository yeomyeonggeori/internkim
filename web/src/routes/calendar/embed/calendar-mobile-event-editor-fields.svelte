<script lang="ts">
	import type { CalendarLocaleText } from '../text';
	import CalendarParticipantSelector from './calendar-participant-selector.svelte';
	import type { CalendarParticipant } from './calendar-participants';
	import type { MobileEventEditorCalendar } from './calendar-mobile-event-editor-types';

	type MobileEventEditorFieldsProps = {
		text: CalendarLocaleText;
		draftText: CalendarLocaleText['draftPopover'];
		calendars: MobileEventEditorCalendar[];
		canEdit: boolean;
		canDelete: boolean;
		title: string;
		startDateKey: string;
		endDateKey: string;
		startTime: string;
		endTime: string;
		allDay: boolean;
		location: string;
		description: string;
		participants: CalendarParticipant[];
		participantCandidates: CalendarParticipant[];
		calendarID: string;
		onParticipantsChange: (participants: CalendarParticipant[]) => void;
		titleInputElement?: HTMLInputElement;
	};

	let {
		text,
		draftText,
		calendars,
		canEdit,
		canDelete,
		title,
		startDateKey,
		endDateKey,
		startTime,
		endTime,
		allDay,
		location,
		description,
		participants = $bindable<CalendarParticipant[]>(),
		participantCandidates,
		calendarID,
		onParticipantsChange,
		titleInputElement = $bindable<HTMLInputElement | undefined>()
	}: MobileEventEditorFieldsProps = $props();
</script>

<div class="df-mobile-event-drawer-body">
	<label class="mobile-event-title-row">
		<span class="mobile-event-color-dot" aria-hidden="true"></span>
		<input
			bind:this={titleInputElement}
			name="title"
			value={title}
			placeholder={text.newEvent}
			aria-label={text.newEvent}
			autocomplete="off"
			readonly={!canEdit}
			data-mobile-editor-field="title"
		/>
	</label>

	<div class="mobile-event-field mobile-event-date-field" data-mobile-editor-picker-field>
		<span class="mobile-event-field-label">{draftText.startDate}</span>
		<input
			type="date"
			value={startDateKey}
			aria-label={draftText.startDate}
			disabled={!canEdit}
			data-mobile-editor-field="startDate"
		/>
		{#if !allDay}
			<input
				type="time"
				value={startTime}
				aria-label={draftText.startTime}
				disabled={!canEdit}
				data-mobile-editor-field="startTime"
			/>
		{/if}
	</div>

	<div class="mobile-event-field mobile-event-date-field" data-mobile-editor-picker-field>
		<span class="mobile-event-field-label">{draftText.endDate}</span>
		<input
			type="date"
			value={endDateKey}
			aria-label={draftText.endDate}
			disabled={!canEdit}
			data-mobile-editor-field="endDate"
		/>
		{#if !allDay}
			<input
				type="time"
				value={endTime}
				aria-label={draftText.endTime}
				disabled={!canEdit}
				data-mobile-editor-field="endTime"
			/>
		{/if}
	</div>

	<label class="mobile-event-field mobile-event-switch-field" data-mobile-editor-switch-field>
		<span class="mobile-event-field-label">{text.allDay}</span>
		<input
			type="checkbox"
			checked={allDay}
			aria-label={text.allDay}
			disabled={!canEdit}
			data-mobile-editor-field="allDay"
		/>
		<span class="mobile-event-switch" aria-hidden="true"></span>
	</label>

	<label class="mobile-event-field">
		<span class="mobile-event-field-label">{text.conflictField.location}</span>
		<input
			value={location}
			aria-label={text.conflictField.location}
			autocomplete="off"
			readonly={!canEdit}
			data-mobile-editor-field="location"
		/>
	</label>

	<label class="mobile-event-field">
		<span class="mobile-event-field-label">{text.conflictField.description}</span>
		<textarea
			value={description}
			aria-label={text.conflictField.description}
			rows="4"
			readonly={!canEdit}
			data-mobile-editor-field="description"
		></textarea>
	</label>

	<div class="mobile-event-field mobile-event-participants-field">
		<span class="mobile-event-field-label">{draftText.participants}</span>
		<CalendarParticipantSelector
			bind:participants
			candidates={participantCandidates}
			label={draftText.participants}
			placeholder={draftText.participantsPlaceholder}
			removeLabel={draftText.removeParticipantAction}
			disabled={!canEdit}
			onChange={onParticipantsChange}
		/>
	</div>

	{#if calendars.length > 0}
		<label class="mobile-event-field">
			<span class="mobile-event-field-label">{draftText.calendar}</span>
			<select
				value={calendarID}
				aria-label={draftText.calendar}
				disabled={!canEdit}
				data-mobile-editor-field="calendar"
			>
				{#each calendars as calendar (calendar.id)}
					<option value={calendar.id}>{calendar.name}</option>
				{/each}
			</select>
		</label>
	{/if}

	{#if canDelete}
		<footer class="mobile-event-delete-footer">
			<button type="button" class="mobile-event-delete-row" data-mobile-editor-action="delete">
				{draftText.delete}
			</button>
		</footer>
	{/if}
</div>

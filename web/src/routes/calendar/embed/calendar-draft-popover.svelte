<!-- 캘린더 초안 일정 입력 팝오버를 렌더링합니다. -->
	<script lang="ts">
		import { isDraftPopoverValid, type DraftPopoverState } from './calendar-draft-popover-state';

	type CalendarOption = {
		id: string;
		name: string;
	};

	type DraftPopoverText = {
		title: string;
		allDay: string;
		location: string;
		description: string;
		calendar: string;
		cancel: string;
		complete: string;
		delete: string;
		startDate: string;
		endDate: string;
		startTime: string;
		endTime: string;
	};

	type Props = {
		popover: DraftPopoverState;
		calendarOptions: CalendarOption[];
		text: DraftPopoverText;
		isSaving: boolean;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
		savePopover: () => void;
		cancelPopover: () => void;
		deletePopover: () => void;
	};

	let {
		popover,
		calendarOptions,
		text,
		isSaving,
		updatePopover,
		savePopover,
		cancelPopover,
		deletePopover
	}: Props = $props();

	const popoverStyle = $derived(`left: ${popover.position.left}px; top: ${popover.position.top}px;`);
	const canSavePopover = $derived(isDraftPopoverValid(popover));

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement || event.currentTarget instanceof HTMLTextAreaElement
			? event.currentTarget.value
			: '';
	}

	function selectValue(event: Event): string {
		return event.currentTarget instanceof HTMLSelectElement ? event.currentTarget.value : '';
	}
</script>

<section class="calendar-draft-popover" style={popoverStyle}>
	<label class="draft-field draft-title-field">
		<span>{text.title}</span>
		<input
			value={popover.title}
			aria-label={text.title}
			autocomplete="off"
			oninput={(event) => updatePopover({ title: inputValue(event) })}
			onkeydown={(event) => {
				if (event.key === 'Enter') savePopover();
				if (event.key === 'Escape') cancelPopover();
			}}
		/>
	</label>

	<div class="draft-time-grid">
		<label class="draft-field">
			<span>{text.startDate}</span>
			<input
				type="date"
				value={popover.dateKey}
				aria-label={text.startDate}
				oninput={(event) => updatePopover({ dateKey: inputValue(event) })}
			/>
		</label>
		<label class="draft-field">
			<span>{text.endDate}</span>
			<input
				type="date"
				value={popover.endDateKey}
				aria-label={text.endDate}
				oninput={(event) => updatePopover({ endDateKey: inputValue(event) })}
			/>
		</label>
		{#if !popover.allDay}
			<label class="draft-field">
				<span>{text.startTime}</span>
				<input
					type="time"
					value={popover.startTime}
					aria-label={text.startTime}
					oninput={(event) => updatePopover({ startTime: inputValue(event) })}
				/>
			</label>
			<label class="draft-field">
				<span>{text.endTime}</span>
				<input
					type="time"
					value={popover.endTime}
					aria-label={text.endTime}
					oninput={(event) => updatePopover({ endTime: inputValue(event) })}
				/>
			</label>
		{/if}
	</div>

	<label class="draft-check-field">
		<input
			type="checkbox"
			checked={popover.allDay}
			aria-label={text.allDay}
			onchange={(event) =>
				updatePopover({ allDay: event.currentTarget instanceof HTMLInputElement ? event.currentTarget.checked : false })}
		/>
		<span>{text.allDay}</span>
	</label>

	<label class="draft-field">
		<span>{text.location}</span>
		<input
			value={popover.location}
			aria-label={text.location}
			autocomplete="off"
			oninput={(event) => updatePopover({ location: inputValue(event) })}
		/>
	</label>

	<label class="draft-field">
		<span>{text.description}</span>
		<textarea
			value={popover.description}
			aria-label={text.description}
			rows="3"
			oninput={(event) => updatePopover({ description: inputValue(event) })}
		></textarea>
	</label>

	<label class="draft-field">
		<span>{text.calendar}</span>
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

	<footer class="draft-actions">
		{#if popover.mode === 'edit'}
			<button type="button" class="draft-delete-button" disabled={isSaving} onclick={deletePopover}>
				{text.delete}
			</button>
		{/if}
		<div class="draft-action-spacer"></div>
		<button type="button" class="draft-secondary-button" disabled={isSaving} onclick={cancelPopover}>
			{text.cancel}
		</button>
		<button type="button" class="draft-primary-button" disabled={isSaving || !canSavePopover} onclick={savePopover}>
			{text.complete}
		</button>
	</footer>
</section>

<style>
	.calendar-draft-popover {
		position: fixed;
		z-index: 80;
		width: min(360px, calc(100vw - 32px));
		max-height: calc(100vh - 32px);
		overflow: auto;
		border: 1px solid #d4d4d8;
		border-radius: 8px;
		background: #ffffff;
		box-shadow: 0 24px 60px rgb(15 23 42 / 0.18);
		padding: 14px;
		color: #18181b;
	}

	.draft-field {
		display: grid;
		gap: 6px;
		margin-bottom: 10px;
		font-size: 12px;
		font-weight: 700;
		color: #52525b;
	}

	.draft-title-field input {
		font-size: 16px;
		font-weight: 700;
	}

	.draft-field input,
	.draft-field textarea,
	.draft-field select {
		width: 100%;
		border: 1px solid #e4e4e7;
		border-radius: 6px;
		background: #ffffff;
		padding: 8px 10px;
		color: #18181b;
		font-size: 13px;
		font-weight: 500;
		outline: none;
	}

	.draft-field input:focus,
	.draft-field textarea:focus,
	.draft-field select:focus {
		border-color: oklch(0.55 0.19 255);
		box-shadow: 0 0 0 3px rgb(59 130 246 / 0.16);
	}

	.draft-time-grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 8px;
	}

	.draft-check-field {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin: 0 0 12px;
		color: #27272a;
		font-size: 13px;
		font-weight: 700;
	}

	.draft-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 12px;
	}

	.draft-action-spacer {
		flex: 1;
	}

	.draft-primary-button,
	.draft-secondary-button,
	.draft-delete-button {
		height: 34px;
		border-radius: 6px;
		padding: 0 12px;
		font-size: 13px;
		font-weight: 800;
	}

	.draft-primary-button {
		border: 1px solid oklch(0.55 0.19 255);
		background: oklch(0.55 0.19 255);
		color: #ffffff;
	}

	.draft-primary-button:disabled {
		opacity: 0.45;
	}

	.draft-secondary-button {
		border: 1px solid #e4e4e7;
		background: #ffffff;
		color: #27272a;
	}

	.draft-delete-button {
		border: 1px solid #fecaca;
		background: #fff1f2;
		color: #be123c;
	}

	:global(html.dark) .calendar-draft-popover {
		border-color: #3f3f46;
		background: #18181b;
		color: #f4f4f5;
	}

	:global(html.dark) .draft-field,
	:global(html.dark) .draft-check-field {
		color: #d4d4d8;
	}

	:global(html.dark) .draft-field input,
	:global(html.dark) .draft-field textarea,
	:global(html.dark) .draft-field select,
	:global(html.dark) .draft-secondary-button {
		border-color: #3f3f46;
		background: #09090b;
		color: #f4f4f5;
	}
</style>

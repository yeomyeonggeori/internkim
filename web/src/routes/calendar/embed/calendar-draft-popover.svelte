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

	const popoverStyle = $derived(
		`left: ${popover.position.left}px; top: ${popover.position.top}px; width: ${popover.position.width}px; --draft-popover-arrow-top: ${popover.position.arrowTop}px;`
	);
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

<section class="calendar-draft-popover" class:popover-arrow-right={popover.position.arrowSide === 'right'} style={popoverStyle}>
	<label class="draft-popover-title-row">
		<span class="draft-popover-color-dot" aria-hidden="true"></span>
		<input
			value={popover.title}
			aria-label={text.title}
			placeholder={text.title}
			autocomplete="off"
			oninput={(event) => updatePopover({ title: inputValue(event) })}
			onkeydown={(event) => {
				if (event.key === 'Enter') savePopover();
				if (event.key === 'Escape') cancelPopover();
			}}
		/>
		<button type="button" class="draft-popover-icon-button" aria-label={text.cancel} onclick={cancelPopover}>
			×
		</button>
	</label>

	<div class="draft-popover-field">
		<span class="draft-popover-field-label">{text.startDate}</span>
		<div class="draft-popover-datetime-inputs">
			<label class="draft-popover-date-time-row">
				<span>{text.startDate}</span>
				<input
					type="date"
					value={popover.dateKey}
					aria-label={text.startDate}
					oninput={(event) => updatePopover({ dateKey: inputValue(event) })}
				/>
				{#if !popover.allDay}
					<input
						type="time"
						value={popover.startTime}
						aria-label={text.startTime}
						oninput={(event) => updatePopover({ startTime: inputValue(event) })}
					/>
				{/if}
			</label>
			<label class="draft-popover-date-time-row">
				<span>{text.endDate}</span>
				<input
					type="date"
					value={popover.endDateKey}
					aria-label={text.endDate}
					oninput={(event) => updatePopover({ endDateKey: inputValue(event) })}
				/>
				{#if !popover.allDay}
					<input
						type="time"
						value={popover.endTime}
						aria-label={text.endTime}
						oninput={(event) => updatePopover({ endTime: inputValue(event) })}
					/>
				{/if}
			</label>
			<label class="draft-popover-all-day-toggle">
				<input
					type="checkbox"
					checked={popover.allDay}
					aria-label={text.allDay}
					onchange={(event) =>
						updatePopover({ allDay: event.currentTarget instanceof HTMLInputElement ? event.currentTarget.checked : false })}
				/>
				<span>{text.allDay}</span>
			</label>
		</div>
	</div>

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

	<footer class="draft-popover-footer">
		{#if popover.mode === 'edit'}
			<button type="button" class="draft-popover-delete" disabled={isSaving} onclick={deletePopover}>
				{text.delete}
			</button>
		{/if}
		<button type="button" class="draft-popover-cancel" disabled={isSaving} onclick={cancelPopover}>
			{text.cancel}
		</button>
		<button type="button" class="draft-popover-complete" disabled={isSaving || !canSavePopover} onclick={savePopover}>
			{text.complete}
		</button>
	</footer>

</section>

<style>
	.calendar-draft-popover {
		position: absolute;
		z-index: 80;
		width: min(540px, calc(100% - 24px));
		max-height: calc(100svh - 24px);
		box-sizing: border-box;
		overflow: visible;
		border: 1px solid rgba(148, 163, 184, 0.34);
		border-radius: 18px;
		background:
			linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(241, 245, 249, 0.86)),
			rgba(248, 250, 252, 0.9);
		padding: 16px;
		color: #1f2937;
		box-shadow:
			0 22px 60px rgba(15, 23, 42, 0.22),
			0 1px 0 rgba(255, 255, 255, 0.8) inset;
		backdrop-filter: blur(18px) saturate(1.35);
	}

	.calendar-draft-popover::before {
		position: absolute;
		top: var(--draft-popover-arrow-top, 42px);
		left: -9px;
		width: 18px;
		height: 16px;
		border-bottom: 1px solid rgba(148, 163, 184, 0.34);
		border-left: 1px solid rgba(148, 163, 184, 0.34);
		background: rgba(248, 250, 252, 0.92);
		content: '';
		transform: rotate(45deg);
	}

	.calendar-draft-popover.popover-arrow-right::before {
		right: -9px;
		left: auto;
		border: 0;
		border-top: 1px solid rgba(148, 163, 184, 0.34);
		border-right: 1px solid rgba(148, 163, 184, 0.34);
	}

	.draft-popover-title-row {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) 30px;
		align-items: center;
		gap: 9px;
		padding: 12px 10px 12px 12px;
		border-radius: 14px;
		background: rgba(241, 245, 249, 0.74);
	}

	.draft-popover-color-dot {
		width: 9px;
		height: 22px;
		border-radius: 999px;
		background: #3b82f6;
		box-shadow: 0 0 0 3px rgb(59 130 246 / 0.18);
	}

	.draft-popover-title-row input {
		min-width: 0;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: #111827;
		font-size: 20px;
		font-weight: 500;
		line-height: 1.2;
		outline: none;
	}

	.draft-popover-title-row input:focus-visible {
		box-shadow: 0 0 0 3px rgb(59 130 246 / 0.16);
	}

	.draft-popover-title-row input::placeholder {
		color: rgba(100, 116, 139, 0.36);
	}

	.draft-popover-icon-button {
		display: inline-flex;
		width: 30px;
		height: 30px;
		align-items: center;
		justify-content: center;
		border-radius: 10px;
		color: #64748b;
		font-size: 20px;
		line-height: 1;
	}

	.draft-popover-icon-button:hover {
		background: rgba(203, 213, 225, 0.68);
		color: #0f172a;
	}

	.draft-popover-field {
		display: grid;
		grid-template-columns: 86px minmax(0, 1fr);
		align-items: start;
		gap: 12px;
		margin-top: 10px;
		padding: 12px;
		border-radius: 14px;
		background: rgba(226, 232, 240, 0.58);
		color: #64748b;
		font-size: 13px;
	}

	.draft-popover-field-label {
		padding-top: 7px;
		color: #334155;
		font-size: 12px;
		font-weight: 700;
	}

	.draft-popover-field input,
	.draft-popover-field textarea,
	.draft-popover-field select {
		min-width: 0;
		width: 100%;
		border: 0;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.72);
		color: #111827;
		font-size: 13px;
		outline: none;
	}

	.draft-popover-field input:focus-visible,
	.draft-popover-field textarea:focus-visible,
	.draft-popover-field select:focus-visible {
		box-shadow:
			inset 0 0 0 1px rgb(59 130 246 / 0.52),
			0 0 0 3px rgb(59 130 246 / 0.14);
	}

	.draft-popover-field input,
	.draft-popover-field select {
		height: 36px;
		padding: 0 10px;
	}

	.draft-popover-field textarea {
		min-height: 92px;
		resize: vertical;
		padding: 8px 10px;
		line-height: 1.45;
	}

	.draft-popover-datetime-inputs {
		display: grid;
		gap: 8px;
	}

	.draft-popover-date-time-row {
		display: grid;
		grid-template-columns: 62px minmax(144px, 1fr) minmax(108px, 0.56fr);
		align-items: center;
		gap: 8px;
	}

	.draft-popover-date-time-row span {
		color: #334155;
		font-size: 12px;
		font-weight: 700;
	}

	.draft-popover-all-day-toggle {
		display: inline-flex;
		width: fit-content;
		min-height: 24px;
		align-items: center;
		gap: 8px;
		color: #334155;
		font-size: 13px;
		font-weight: 700;
		line-height: 16px;
		white-space: nowrap;
	}

	.draft-popover-all-day-toggle input {
		width: 16px;
		height: 16px;
		padding: 0;
		accent-color: #0b57d0;
	}

	.draft-popover-footer {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 10px;
		margin-top: 14px;
	}

	.draft-popover-cancel,
	.draft-popover-complete,
	.draft-popover-delete {
		min-width: 76px;
		height: 38px;
		border-radius: 999px;
		padding: 0 18px;
		font-size: 13px;
		font-weight: 700;
	}

	.draft-popover-delete {
		margin-right: auto;
		border: 1px solid rgba(220, 38, 38, 0.24);
		background: rgba(254, 242, 242, 0.82);
		color: #b91c1c;
	}

	.draft-popover-cancel {
		border: 1px solid rgba(100, 116, 139, 0.28);
		background: rgba(255, 255, 255, 0.72);
		color: #334155;
	}

	.draft-popover-complete {
		background: #0b57d0;
		color: #ffffff;
		box-shadow: 0 10px 20px rgba(11, 87, 208, 0.26);
	}

	.draft-popover-complete:disabled {
		cursor: not-allowed;
		background: #cbd5e1;
		box-shadow: none;
	}

	:global(html.dark) .calendar-draft-popover {
		border-color: #3f3f46;
		background: #18181b;
		color: #f4f4f5;
	}
</style>

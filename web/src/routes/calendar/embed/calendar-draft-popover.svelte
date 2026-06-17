<script lang="ts">
	import type { CalendarAuditRow } from './calendar-audit';
	import CalendarEventAuditCard from './calendar-event-audit-card.svelte';
	import {
		draftPopoverAllDayChanges,
		isDraftPopoverValid,
		type DraftPopoverState
	} from './calendar-draft-popover-state';
	import './calendar-draft-popover.css';

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
		auditEmpty: string;
	};

	type Props = {
		popover: DraftPopoverState;
		calendarOptions: CalendarOption[];
		auditRows: CalendarAuditRow[];
		auditLabel: string;
		text: DraftPopoverText;
		isSaving: boolean;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
		repositionPopover: (size: { width: number; height: number }) => void;
		savePopover: () => void;
		cancelPopover: () => void;
		deletePopover: () => void;
	};

	let {
		popover,
		calendarOptions,
		auditRows,
		auditLabel,
		text,
		isSaving,
		updatePopover,
		repositionPopover,
		savePopover,
		cancelPopover,
		deletePopover
	}: Props = $props();

	let popoverElement: HTMLElement | null = null;
	let measurementFrame: number | null = null;
	let lastMeasuredSizeKey = '';

	const popoverStyle = $derived(
		`left: ${popover.position.left}px; top: ${popover.position.top}px; width: ${popover.position.width}px; --draft-popover-arrow-top: ${popover.position.arrowTop}px;`
	);
	const canSavePopover = $derived(isDraftPopoverValid(popover));
	const auditEmptyText = $derived(popover.mode === 'edit' ? text.auditEmpty : '');

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement || event.currentTarget instanceof HTMLTextAreaElement
			? event.currentTarget.value
			: '';
	}

	function selectValue(event: Event): string {
		return event.currentTarget instanceof HTMLSelectElement ? event.currentTarget.value : '';
	}

	function allDayChanges(event: Event): Partial<DraftPopoverState> {
		const allDay = event.currentTarget instanceof HTMLInputElement ? event.currentTarget.checked : false;
		return draftPopoverAllDayChanges(popover, allDay);
	}

	function measurePopover(element: HTMLElement): { destroy: () => void } {
		popoverElement = element;
		const resizeObserver = new ResizeObserver(() => schedulePopoverMeasurement(element));
		resizeObserver.observe(element);
		schedulePopoverMeasurement(element);
		return {
			destroy: () => {
				popoverElement = null;
				resizeObserver.disconnect();
				if (measurementFrame !== null) cancelAnimationFrame(measurementFrame);
			}
		};
	}

	function schedulePopoverMeasurement(element: HTMLElement): void {
		if (!popover.position.isReady) return;
		if (measurementFrame !== null) cancelAnimationFrame(measurementFrame);
		measurementFrame = requestAnimationFrame(() => {
			measurementFrame = null;
			if (!popover.position.isReady) return;
			const rectangle = element.getBoundingClientRect();
			const sizeKey = `${popover.eventID}:${Math.round(rectangle.width)}:${Math.round(rectangle.height)}`;
			if (sizeKey === lastMeasuredSizeKey) return;
			lastMeasuredSizeKey = sizeKey;
			repositionPopover({ width: rectangle.width, height: rectangle.height });
		});
	}

	$effect(() => {
		popover.title;
		popover.allDay;
		popover.description;
		popover.location;
		popover.position.isReady;
		auditRows;
		if (!popoverElement) return;
		schedulePopoverMeasurement(popoverElement);
	});
</script>

<section
	use:measurePopover
	class="calendar-draft-popover"
	class:draft-popover-pending={!popover.position.isReady}
	class:popover-arrow-right={popover.position.arrowSide === 'right'}
	style={popoverStyle}
>
	<div class="draft-popover-scroll">
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
						onchange={(event) => updatePopover(allDayChanges(event))}
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

		<CalendarEventAuditCard rows={auditRows} label={auditLabel} emptyText={auditEmptyText} />

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
	</div>

</section>

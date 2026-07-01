<script lang="ts">
	import type { CalendarAuditRow } from './calendar-audit';
	import { isDraftPopoverValid, type DraftPopoverState } from './calendar-draft-popover-state';
	import type { DraftPopoverText } from './calendar-draft-popover-text';
	import type { CalendarParticipant } from './calendar-participants';
	import CalendarDraftPopoverBody from './calendar-draft-popover-body.svelte';
	import CalendarDraftPopoverFooter from './calendar-draft-popover-footer.svelte';
	import CalendarDraftPopoverTitleRow from './calendar-draft-popover-title-row.svelte';
	import './calendar-draft-popover.css';

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
		participantCandidates,
		auditRows,
		auditLabel,
		localeCode,
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
	let canScrollUp = $state(false);
	let canScrollDown = $state(false);

	const popoverStyle = $derived(
		`left: ${popover.position.left}px; top: ${popover.position.top}px; width: ${popover.position.width}px; --draft-popover-arrow-top: ${popover.position.arrowTop}px;`
	);
	const canSavePopover = $derived(isDraftPopoverValid(popover));

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
			const sizeKey = [
				popover.eventID,
				Math.round(rectangle.width),
				Math.round(rectangle.height),
				Math.round(popover.position.left),
				Math.round(popover.position.top),
				Math.round(popover.position.width),
				Math.round(popover.position.arrowTop)
			].join(':');
			if (sizeKey === lastMeasuredSizeKey) return;
			lastMeasuredSizeKey = sizeKey;
			repositionPopover({ width: rectangle.width, height: rectangle.height });
		});
	}

	function setScrollState(nextCanScrollUp: boolean, nextCanScrollDown: boolean): void {
		canScrollUp = nextCanScrollUp;
		canScrollDown = nextCanScrollDown;
	}

	$effect(() => {
		popover.title;
		popover.allDay;
		popover.description;
		popover.location;
		popover.participants;
		popover.position.left;
		popover.position.top;
		popover.position.width;
		popover.position.arrowTop;
		popover.position.arrowSide;
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
	class:draft-popover-can-scroll-up={canScrollUp}
	class:draft-popover-can-scroll-down={canScrollDown}
	style={popoverStyle}
>
	<CalendarDraftPopoverTitleRow {popover} {text} {updatePopover} {savePopover} {cancelPopover} />

	<CalendarDraftPopoverBody
		{popover}
		{calendarOptions}
		{participantCandidates}
		{auditRows}
		{auditLabel}
		{localeCode}
		{text}
		{updatePopover}
		{setScrollState}
	/>

	<CalendarDraftPopoverFooter
		{popover}
		{text}
		{isSaving}
		{canSavePopover}
		{savePopover}
		{cancelPopover}
		{deletePopover}
	/>
</section>

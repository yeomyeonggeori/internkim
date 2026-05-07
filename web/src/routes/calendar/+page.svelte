<script lang="ts">
	import { browser } from '$app/environment';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Separator } from '$lib/components/ui/separator';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import WifiIcon from '@lucide/svelte/icons/wifi';
	import {
		createDayView,
		createEventsPlugin,
		createMonthView,
		createWeekView,
		DayFlowCalendar,
		useCalendarApp,
		ViewType
	} from '@dayflow/svelte';
	import { createEvent, temporalToDate, type Event as DayFlowEvent } from '@dayflow/core';
	import { createDragPlugin } from '@dayflow/plugin-drag';
	import { createSidebarPlugin } from '@dayflow/plugin-sidebar';
	import { onMount } from 'svelte';
	import '@dayflow/core/dist/styles.css';
	import { calendarText } from './text';

	type CalendarEvent = {
		id: string;
		uid: string;
		title: string;
		description: string;
		location: string;
		startISO: string;
		endISO: string;
		timeZone: string;
		isAllDay: boolean;
		color: string;
		updatedAt: string;
	};

	type CalendarEventsResponse = {
		events: CalendarEvent[];
	};

	type CalendarSyncResponse = {
		caldavURL: string;
		icsURL: string;
	};

	type MonthRangeSelection = {
		pointerID: number;
		startDateKey: string;
		endDateKey: string;
		startClientX: number;
		startClientY: number;
		hasMoved: boolean;
	};

	type MonthDateCell = {
		element: HTMLElement;
		dateKey: string;
	};

	type MonthRangePreviewSegment = {
		id: string;
		left: number;
		top: number;
		width: number;
		height: number;
	};

	type CalendarDateParts = {
		year: number;
		month: number;
		day: number;
	};

	const text = createPageText(calendarText);
	const calendarColors = {
		eventColor: '#eff6ff',
		eventSelectedColor: 'rgb(59, 130, 246)',
		lineColor: '#3b82f6',
		textColor: '#1e3a8a'
	};
	const darkCalendarColors = {
		eventColor: 'rgba(30, 64, 175, 0.8)',
		eventSelectedColor: 'rgba(30, 58, 138, 1)',
		lineColor: '#3b82f6',
		textColor: '#dbeafe'
	};
	let visibleStartDate = $state<Date | null>(null);
	let visibleEndDate = $state<Date | null>(null);
	let syncInformation = $state<CalendarSyncResponse | null>(null);
	let isLoading = $state(false);
	let isSaving = $state(false);
	let errorMessage = $state('');
	let statusMessage = $state('');
	let eventCount = $state(0);
	let calendarStageElement = $state<HTMLElement | null>(null);
	let monthRangeSelection = $state<MonthRangeSelection | null>(null);
	let monthRangePreviewSegments = $state<MonthRangePreviewSegment[]>([]);
	let lastMonthRangeCreationTime = 0;

	const calendar = useCalendarApp({
		views: [
			createDayView({ showAllDay: true, timeFormat: '24h' }),
			createWeekView({ showWeekends: true, startOfWeek: 1 }),
			createMonthView({ showWeekNumbers: false })
		],
		defaultView: ViewType.WEEK,
		initialDate: new Date(),
		timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
		switcherMode: 'buttons',
		useCalendarHeader: true,
		useEventDetailDialog: false,
		useEventDetailPanel: true,
		calendars: [
			{
				id: 'internkim',
				name: 'Work',
				colors: calendarColors,
				darkColors: darkCalendarColors,
				isVisible: true
			}
		],
		defaultCalendar: 'internkim',
		theme: { mode: 'auto' },
		plugins: [
			createSidebarPlugin({
				width: 280,
				initialCollapsed: false,
				createCalendarMode: 'modal'
			}),
			createEventsPlugin(),
			createDragPlugin({
				enableDrag: true,
				enableResize: true,
				enableCreate: true,
				enableAllDayCreate: true
			})
		],
		callbacks: {
			onVisibleRangeChange: (startDate, endDate) => loadEvents(startDate, endDate),
			onEventCreate: (event) => saveCreatedEvent(event),
			onEventUpdate: (event) => saveUpdatedEvent(event),
			onEventDelete: (eventID) => deleteEvent(eventID)
		}
	});

	onMount(() => {
		loadSyncInformation();
		if (!visibleStartDate || !visibleEndDate) {
			const today = new Date();
			loadEvents(startOfMonthWindow(today), endOfMonthWindow(today));
		}
		return installMonthRangeCreate();
	});

	$effect(() => {
		monthRangeSelection;
		monthRangePreviewSegments = monthRangePreviewSegmentsFromSelection();
	});

	function startOfMonthWindow(date: Date) {
		return new Date(date.getFullYear(), date.getMonth() - 1, 1);
	}

	function endOfMonthWindow(date: Date) {
		return new Date(date.getFullYear(), date.getMonth() + 2, 1);
	}

	async function loadEvents(startDate: Date, endDate: Date) {
		if (!browser) return;
		visibleStartDate = startDate;
		visibleEndDate = endDate;
		isLoading = true;
		errorMessage = '';
		try {
			const query = new URLSearchParams({
				startISO: startDate.toISOString(),
				endISO: endDate.toISOString()
			});
			const response = await fetch(`/calendar/api/events?${query}`, { credentials: 'include' });
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.error));
			const document = (await response.json()) as CalendarEventsResponse;
			eventCount = document.events.length;
			calendar.app.updateConfig({ events: document.events.map(dayFlowEventFromCalendarEvent) });
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.error;
		} finally {
			isLoading = false;
		}
	}

	async function loadSyncInformation() {
		try {
			const response = await fetch('/calendar/api/sync', { credentials: 'include' });
			if (!response.ok) return;
			syncInformation = (await response.json()) as CalendarSyncResponse;
		} catch {
			syncInformation = null;
		}
	}

	async function rotateSubscriptionURL() {
		isSaving = true;
		errorMessage = '';
		try {
			const response = await fetch('/calendar/api/ics-token', {
				method: 'POST',
				credentials: 'include'
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.saveError));
			syncInformation = (await response.json()) as CalendarSyncResponse;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.saveError;
		} finally {
			isSaving = false;
		}
	}

	async function refreshCalendar() {
		if (!visibleStartDate || !visibleEndDate) return;
		await loadEvents(visibleStartDate, visibleEndDate);
	}

	async function saveCreatedEvent(event: DayFlowEvent) {
		await persistEvent('/calendar/api/events', 'POST', event);
	}

	async function saveUpdatedEvent(event: DayFlowEvent) {
		await persistEvent(`/calendar/api/events/${encodeURIComponent(event.id)}`, 'PUT', event);
	}

	async function persistEvent(path: string, method: string, event: DayFlowEvent) {
		isSaving = true;
		statusMessage = '';
		errorMessage = '';
		try {
			const response = await fetch(path, {
				method,
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(calendarEventPayloadFromDayFlowEvent(event))
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.saveError));
			statusMessage = text.shared;
			await refreshCalendar();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.saveError;
			await refreshCalendar();
		} finally {
			isSaving = false;
		}
	}

	async function deleteEvent(eventID: string) {
		isSaving = true;
		errorMessage = '';
		try {
			const response = await fetch(`/calendar/api/events/${encodeURIComponent(eventID)}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.deleteError));
			await refreshCalendar();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.deleteError;
			await refreshCalendar();
		} finally {
			isSaving = false;
		}
	}

	function dayFlowEventFromCalendarEvent(event: CalendarEvent) {
		return createEvent({
			id: event.id,
			title: event.title,
			description: event.description,
			start: event.isAllDay ? localDateFromISODate(event.startISO) : new Date(event.startISO),
			end: event.isAllDay ? dayBeforeLocalISODate(event.endISO) : new Date(event.endISO),
			allDay: event.isAllDay,
			calendarId: 'internkim',
			meta: {
				uid: event.uid,
				location: event.location,
				color: event.color,
				timeZone: event.timeZone,
				updatedAt: event.updatedAt
			}
		});
	}

	function calendarEventPayloadFromDayFlowEvent(event: DayFlowEvent) {
		const startDate = calendarDateFromDayFlowEventStart(event);
		const endDate = calendarDateFromDayFlowEventEnd(event);
		return {
			title: event.title || 'Untitled event',
			description: event.description ?? '',
			location: typeof event.meta?.location === 'string' ? event.meta.location : '',
			startISO: startDate.toISOString(),
			endISO: endDate.toISOString(),
			timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
			isAllDay: event.allDay ?? false,
			color: calendarColors.lineColor
		};
	}

	function calendarDateFromDayFlowEventStart(event: DayFlowEvent) {
		if (event.allDay) return dateFromCalendarDateParts(calendarDatePartsFromTemporal(event.start));
		return temporalToDate(event.start);
	}

	function calendarDateFromDayFlowEventEnd(event: DayFlowEvent) {
		if (event.allDay) return dateFromCalendarDateParts(nextCalendarDateParts(calendarDatePartsFromTemporal(event.end)));
		return temporalToDate(event.end);
	}

	function calendarDatePartsFromTemporal(value: DayFlowEvent['start']) {
		if (isCalendarDateParts(value)) return value;
		const date = temporalToDate(value);
		return {
			year: date.getFullYear(),
			month: date.getMonth() + 1,
			day: date.getDate()
		};
	}

	function isCalendarDateParts(value: unknown): value is CalendarDateParts {
		if (!value || typeof value !== 'object') return false;
		return 'year' in value && 'month' in value && 'day' in value;
	}

	function dateFromCalendarDateParts(dateParts: CalendarDateParts) {
		return new Date(Date.UTC(dateParts.year, dateParts.month - 1, dateParts.day));
	}

	function localDateFromISODate(isoDate: string) {
		const date = new Date(isoDate);
		return new Date(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate());
	}

	function dayBeforeLocalISODate(isoDate: string) {
		const date = localDateFromISODate(isoDate);
		date.setDate(date.getDate() - 1);
		return date;
	}

	function nextCalendarDateParts(dateParts: CalendarDateParts) {
		const date = new Date(dateParts.year, dateParts.month - 1, dateParts.day);
		date.setDate(date.getDate() + 1);
		return {
			year: date.getFullYear(),
			month: date.getMonth() + 1,
			day: date.getDate()
		};
	}

	function installMonthRangeCreate() {
		if (!browser || !calendarStageElement) return;
		const stageElement = calendarStageElement;
		const handlePointerDown = (event: PointerEvent) => startMonthRangeSelection(event);
		const handlePointerMove = (event: PointerEvent) => moveMonthRangeSelection(event);
		const handlePointerUp = (event: PointerEvent) => finishMonthRangeSelection(event);
		const handleClick = (event: MouseEvent) => suppressMonthRangeClick(event);

		stageElement.addEventListener('pointerdown', handlePointerDown);
		document.addEventListener('pointermove', handlePointerMove);
		document.addEventListener('pointerup', handlePointerUp);
		document.addEventListener('pointercancel', handlePointerUp);
		stageElement.addEventListener('click', handleClick, true);

		return () => {
			stageElement.removeEventListener('pointerdown', handlePointerDown);
			document.removeEventListener('pointermove', handlePointerMove);
			document.removeEventListener('pointerup', handlePointerUp);
			document.removeEventListener('pointercancel', handlePointerUp);
			stageElement.removeEventListener('click', handleClick, true);
		};
	}

	function startMonthRangeSelection(event: PointerEvent) {
		if (event.button !== 0 || monthRangeSelection) return;
		const dateCell = monthDateCellFromEventTarget(event.target);
		if (!dateCell) return;
		monthRangeSelection = {
			pointerID: event.pointerId,
			startDateKey: dateCell.dateKey,
			endDateKey: dateCell.dateKey,
			startClientX: event.clientX,
			startClientY: event.clientY,
			hasMoved: false
		};
	}

	function moveMonthRangeSelection(event: PointerEvent) {
		if (!monthRangeSelection || monthRangeSelection.pointerID !== event.pointerId) return;
		const hasMoved = monthRangeSelection.hasMoved || hasPointerMoved(monthRangeSelection, event);
		const dateCell = monthDateCellFromPoint(event.clientX, event.clientY);
		if (!dateCell && monthRangeSelection.hasMoved === hasMoved) return;
		if (hasMoved) event.preventDefault();
		monthRangeSelection = {
			...monthRangeSelection,
			endDateKey: dateCell?.dateKey ?? monthRangeSelection.endDateKey,
			hasMoved
		};
	}

	function finishMonthRangeSelection(event: PointerEvent) {
		if (!monthRangeSelection || monthRangeSelection.pointerID !== event.pointerId) return;
		const selection = monthRangeSelection;
		monthRangeSelection = null;
		monthRangePreviewSegments = [];
		if (!selection.hasMoved) return;
		event.preventDefault();
		event.stopPropagation();
		lastMonthRangeCreationTime = Date.now();
		createMonthRangeEvent(selection);
	}

	function suppressMonthRangeClick(event: MouseEvent) {
		if (Date.now() - lastMonthRangeCreationTime > 350) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
	}

	function hasPointerMoved(selection: MonthRangeSelection, event: PointerEvent) {
		return Math.hypot(event.clientX - selection.startClientX, event.clientY - selection.startClientY) > 8;
	}

	function monthDateCellFromEventTarget(target: EventTarget | null) {
		if (calendar.currentView !== ViewType.MONTH) return null;
		if (!(target instanceof Element)) return null;
		if (isMonthRangeIgnoredTarget(target)) return null;
		return monthDateCellFromElement(target);
	}

	function monthDateCellFromPoint(clientX: number, clientY: number) {
		const element = document.elementFromPoint(clientX, clientY);
		if (!element) return null;
		return monthDateCellFromElement(element);
	}

	function monthDateCellFromElement(element: Element): MonthDateCell | null {
		const dateCell = element.closest('.df-month-day-cell[data-date]');
		if (!(dateCell instanceof HTMLElement)) return null;
		const dateKey = dateCell.dataset.date;
		if (!dateKey) return null;
		return { element: dateCell, dateKey };
	}

	function isMonthRangeIgnoredTarget(target: Element) {
		return Boolean(
			target.closest(
				'.df-event, .df-month-more-events, .df-event-detail-panel, .df-dialog-container, .sync-popover, [data-range-picker-popup], [data-calendar-picker-dropdown]'
			)
		);
	}

	function createMonthRangeEvent(selection: MonthRangeSelection) {
		const [startDateKey, endDateKey] = orderedDateKeys(selection.startDateKey, selection.endDateKey);
		const event = createEvent({
			id: `range-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
			title: 'New Event',
			start: dateFromDateKey(startDateKey),
			end: dateFromDateKey(endDateKey),
			allDay: true,
			calendarId: 'internkim'
		});
		calendar.addEvent(event);
	}

	function orderedDateKeys(firstDateKey: string, secondDateKey: string) {
		if (firstDateKey <= secondDateKey) return [firstDateKey, secondDateKey];
		return [secondDateKey, firstDateKey];
	}

	function dateFromDateKey(dateKey: string) {
		const [year = '0', month = '1', day = '1'] = dateKey.split('-');
		return new Date(Number(year), Number(month) - 1, Number(day));
	}

	function monthRangePreviewSegmentsFromSelection() {
		if (!browser || !calendarStageElement) return [];
		const selectedRange = selectedMonthRange();
		if (!selectedRange) return [];
		const stageRectangle = calendarStageElement.getBoundingClientRect();
		const selectedDateCells = selectedMonthDateCells(selectedRange);
		const dateCellsByWeek = new Map<HTMLElement, HTMLElement[]>();
		for (const dateCell of selectedDateCells) {
			const weekElement = dateCell.closest('.df-month-week-grid');
			if (!(weekElement instanceof HTMLElement)) continue;
			dateCellsByWeek.set(weekElement, [...(dateCellsByWeek.get(weekElement) ?? []), dateCell]);
		}
		return Array.from(dateCellsByWeek.entries()).flatMap(([weekElement, dateCells], index) =>
			monthRangePreviewSegmentFromCells(weekElement, dateCells, stageRectangle, index)
		);
	}

	function selectedMonthDateCells(selectedRange: { startDateKey: string; endDateKey: string }) {
		if (!calendarStageElement) return [];
		return Array.from(calendarStageElement.querySelectorAll('.df-month-day-cell[data-date]')).filter(
			(element): element is HTMLElement => isSelectedMonthDateCell(element, selectedRange)
		);
	}

	function isSelectedMonthDateCell(element: Element, selectedRange: { startDateKey: string; endDateKey: string }) {
		if (!(element instanceof HTMLElement)) return false;
		const dateKey = element.dataset.date ?? '';
		return dateKey >= selectedRange.startDateKey && dateKey <= selectedRange.endDateKey;
	}

	function monthRangePreviewSegmentFromCells(
		weekElement: HTMLElement,
		dateCells: HTMLElement[],
		stageRectangle: DOMRect,
		index: number
	) {
		const sortedDateCells = [...dateCells].sort((firstDateCell, secondDateCell) =>
			(firstDateCell.dataset.date ?? '').localeCompare(secondDateCell.dataset.date ?? '')
		);
		const firstDateCell = sortedDateCells[0];
		const lastDateCell = sortedDateCells[sortedDateCells.length - 1];
		if (!firstDateCell || !lastDateCell) return [];
		const firstDateCellRectangle = firstDateCell.getBoundingClientRect();
		const lastDateCellRectangle = lastDateCell.getBoundingClientRect();
		const weekRectangle = weekElement.getBoundingClientRect();
		return [
			{
				id: `${firstDateCell.dataset.date ?? index}-${lastDateCell.dataset.date ?? index}`,
				left: firstDateCellRectangle.left - stageRectangle.left + 4,
				top: weekRectangle.top - stageRectangle.top + 34,
				width: Math.max(28, lastDateCellRectangle.right - firstDateCellRectangle.left - 8),
				height: 18
			}
		];
	}

	function selectedMonthRange() {
		if (!monthRangeSelection || !monthRangeSelection.hasMoved) return null;
		const [startDateKey, endDateKey] = orderedDateKeys(monthRangeSelection.startDateKey, monthRangeSelection.endDateKey);
		return { startDateKey, endDateKey };
	}

	async function responseErrorMessage(response: Response, fallback: string) {
		const message = (await response.text()).trim();
		if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
		return message;
	}
</script>

<svelte:head>
	<title>{text.title} · intern kim</title>
</svelte:head>

<main class="min-h-screen bg-background text-foreground">
	<div bind:this={calendarStageElement} class="calendar-stage">
		<DayFlowCalendar {calendar} />

		{#each monthRangePreviewSegments as segment (segment.id)}
			<div
				class="month-range-preview"
				style={`left: ${segment.left}px; top: ${segment.top}px; width: ${segment.width}px; height: ${segment.height}px;`}
			>
				<span class="month-range-preview-dot"></span>
				<span class="month-range-preview-title">New Event</span>
			</div>
		{/each}

		<details class="sync-popover">
			<summary>
				<WifiIcon class="size-4" />
				<span>{text.syncTitle}</span>
			</summary>
			<div class="sync-body">
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{:else if isLoading}
					<p class="text-sm text-muted-foreground">{text.loading}</p>
				{:else if eventCount === 0}
					<p class="text-sm text-muted-foreground">{text.empty}</p>
				{:else if statusMessage}
					<p class="text-sm text-muted-foreground">{statusMessage}</p>
				{/if}

				<Separator />

				<div class="space-y-3">
					<div class="space-y-1.5">
						<p class="text-xs font-medium uppercase text-muted-foreground">{text.caldav}</p>
						<div class="flex items-center gap-2">
							<code class="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1.5 text-xs">
								{syncInformation?.caldavURL ?? ''}
							</code>
							<CopyButton text={syncInformation?.caldavURL ?? ''} variant="outline" size="icon" disabled={!syncInformation?.caldavURL} />
						</div>
					</div>

					<div class="space-y-1.5">
						<p class="text-xs font-medium uppercase text-muted-foreground">{text.ics}</p>
						<div class="flex items-center gap-2">
							<code class="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1.5 text-xs">
								{syncInformation?.icsURL ?? ''}
							</code>
							<CopyButton text={syncInformation?.icsURL ?? ''} variant="outline" size="icon" disabled={!syncInformation?.icsURL} />
						</div>
					</div>
				</div>

				<Button variant="outline" class="w-full gap-2" onclick={rotateSubscriptionURL} disabled={isSaving}>
					<RotateCwIcon class={cn('size-4', isSaving && 'animate-spin')} />
					{text.rotate}
				</Button>
			</div>
		</details>
	</div>
</main>

<style>
	.calendar-stage {
		position: relative;
		min-height: 100vh;
		padding: 16px;
		background: var(--df-color-background, hsl(var(--background)));
	}

	.calendar-stage :global(.df-calendar-container) {
		width: 100%;
		--df-calendar-height: calc(100vh - 32px);
	}

	.month-range-preview {
		position: absolute;
		z-index: 15;
		display: flex;
		align-items: center;
		gap: 6px;
		overflow: hidden;
		border-radius: 4px;
		background: rgb(59 130 246 / 0.96);
		padding: 0 8px;
		color: white;
		font-size: 12px;
		font-weight: 600;
		line-height: 1;
		pointer-events: none;
		box-shadow:
			0 1px 2px rgb(15 23 42 / 0.18),
			inset 0 0 0 1px rgb(255 255 255 / 0.18);
	}

	.month-range-preview-dot {
		width: 6px;
		height: 6px;
		flex: 0 0 auto;
		border-radius: 9999px;
		background: rgb(219 234 254);
	}

	.month-range-preview-title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.sync-popover {
		position: absolute;
		right: 32px;
		bottom: 32px;
		z-index: 20;
		width: min(360px, calc(100vw - 64px));
	}

	.sync-popover summary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		float: right;
		height: 36px;
		cursor: pointer;
		list-style: none;
		border: 1px solid var(--df-color-border, hsl(var(--border)));
		border-radius: 9999px;
		background: var(--df-color-card, hsl(var(--card)));
		padding: 0 14px;
		font-size: 14px;
		font-weight: 500;
		box-shadow:
			0 10px 15px -3px rgb(0 0 0 / 0.1),
			0 4px 6px -4px rgb(0 0 0 / 0.1);
	}

	.sync-popover summary::-webkit-details-marker {
		display: none;
	}

	.sync-body {
		clear: both;
		margin-top: 44px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		border: 1px solid var(--df-color-border, hsl(var(--border)));
		border-radius: 12px;
		background: var(--df-color-card, hsl(var(--card)));
		padding: 16px;
		box-shadow:
			0 20px 25px -5px rgb(0 0 0 / 0.1),
			0 8px 10px -6px rgb(0 0 0 / 0.1);
	}

	@media (max-width: 767px) {
		.calendar-stage {
			padding: 0;
		}

		.calendar-stage :global(.df-calendar-container) {
			border-radius: 0;
			--df-calendar-height: 100vh;
		}

		.sync-popover {
			right: 16px;
			bottom: 16px;
			width: calc(100vw - 32px);
		}
	}
</style>

<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import { CalendarDate, getLocalTimeZone, type DateValue } from '@internationalized/date';
	import type { DateRange } from 'bits-ui';
	import { Button } from '$lib/components/ui/button';
	import { Calendar } from '$lib/components/ui/calendar';
	import { RangeCalendar } from '$lib/components/ui/range-calendar';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Popover from '$lib/components/ui/popover';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import { calendarDateTimeRangeChangesForStart } from './calendar-date-time-range';
	import { isDraftPopoverValid, type DraftPopoverState } from './calendar-draft-popover-state';
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
		dialogLabel: string;
		localeCode: string;
		text: DraftPopoverText;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
		savePopover: () => void;
		cancelPopover: () => void;
		deletePopover: () => void;
	};

	let {
		popover,
		calendarOptions,
		participantCandidates,
		dialogLabel,
		localeCode,
		text,
		updatePopover,
		savePopover,
		cancelPopover,
		deletePopover
	}: Props = $props();

	const anchor = $derived(popover?.anchor ?? null);
	const anchorElement = $derived({
		getBoundingClientRect: () => {
			const originElement = anchor?.originElement;
			if (originElement?.isConnected) return originElement.getBoundingClientRect();
			return new DOMRect(anchor?.clientX ?? window.innerWidth / 2, anchor?.clientY ?? window.innerHeight / 3, 1, 1);
		}
	});
	const canSavePopover = $derived(isDraftPopoverValid(popover));
	const selectedCalendarName = $derived(
		calendarOptions.find((option) => option.id === popover.calendarID)?.name ?? popover.calendarID
	);

	function changeStart(changes: { startDateKey?: string; startTime?: string }): void {
		const range = calendarDateTimeRangeChangesForStart(
			{
				startDateKey: popover.dateKey,
				endDateKey: popover.endDateKey,
				startTime: popover.startTime,
				endTime: popover.endTime,
				allDay: popover.allDay
			},
			changes
		);
		updatePopover({
			dateKey: range.startDateKey,
			endDateKey: range.endDateKey,
			startTime: range.startTime,
			endTime: range.endTime
		});
	}

	let isStartDatePickerOpen = $state(false);
	let isEndDatePickerOpen = $state(false);
	let isRangePickerOpen = $state(false);

	const startCalendarDate = $derived(calendarDateFromDateKey(popover.dateKey));
	const endCalendarDate = $derived(calendarDateFromDateKey(popover.endDateKey));
	let pickedDateRange = $state<DateRange>({ start: undefined, end: undefined });
	const dateRangeLabel = $derived(
		popover.dateKey === popover.endDateKey
			? formatDateKey(popover.dateKey)
			: `${formatDateKey(popover.dateKey)} – ${formatDateKey(popover.endDateKey)}`
	);

	function calendarDateFromDateKey(value: string): CalendarDate | undefined {
		const [year, month, day] = value.split('-').map(Number);
		if (!year || !month || !day) return undefined;
		return new CalendarDate(year, month, day);
	}

	function dateKeyFromCalendarDate(value: DateValue): string {
		return `${value.year}-${String(value.month).padStart(2, '0')}-${String(value.day).padStart(2, '0')}`;
	}

	function formatDateKey(value: string): string {
		const calendarDate = calendarDateFromDateKey(value);
		if (!calendarDate) return value;
		return calendarDate.toDate(getLocalTimeZone()).toLocaleDateString(localeCode, {
			year: 'numeric',
			month: 'long',
			day: 'numeric'
		});
	}

	function openDateRangePicker(isOpen: boolean): void {
		if (!isOpen) return;
		pickedDateRange = { start: startCalendarDate, end: endCalendarDate };
	}

	function changeDateRange(range: DateRange | undefined): void {
		if (!range?.start) return;
		const startKey = dateKeyFromCalendarDate(range.start);
		updatePopover({
			dateKey: startKey,
			endDateKey: range.end ? dateKeyFromCalendarDate(range.end) : startKey
		});
		if (range.end) isRangePickerOpen = false;
	}

	function changeStartDate(value: DateValue | undefined): void {
		if (!value) return;
		changeStart({ startDateKey: dateKeyFromCalendarDate(value) });
		isStartDatePickerOpen = false;
	}

	function changeEndDate(value: DateValue | undefined): void {
		if (!value) return;
		updatePopover({ endDateKey: dateKeyFromCalendarDate(value) });
		isEndDatePickerOpen = false;
	}

	function saveOnEnter(event: KeyboardEvent): void {
		if (event.key !== 'Enter' || event.isComposing || !canSavePopover) return;
		event.preventDefault();
		savePopover();
	}
</script>

<Popover.Root open onOpenChange={(isOpen) => !isOpen && cancelPopover()}>
	<Popover.Content
		customAnchor={anchorElement}
		side={anchor?.originElement ? 'bottom' : 'right'}
		align={anchor?.originElement ? 'end' : 'start'}
		collisionPadding={12}
		interactOutsideBehavior="ignore"
		onOpenAutoFocus={(event) => {
			if (popover.mode === 'edit') event.preventDefault();
		}}
		aria-label={dialogLabel}
		class="max-h-[min(30rem,var(--bits-popover-content-available-height))] w-80 gap-0 overflow-y-auto p-0"
	>
		<div class="border-border/50 border-b">
			<Input
				value={popover.title}
				aria-label={text.title}
				placeholder={text.titlePlaceholder}
				autocomplete="off"
				class="h-10 rounded-none border-0 px-3 text-sm font-semibold shadow-none focus-visible:ring-0"
				oninput={(event) => updatePopover({ title: event.currentTarget.value })}
				onkeydown={saveOnEnter}
			/>
		</div>

		<div class="grid gap-1.5 p-2">
			<div class="flex items-center gap-2 px-1">
				<Label for="draft-all-day" class="text-muted-foreground font-normal">{text.allDay}</Label>
				<Switch
					id="draft-all-day"
					class="ml-auto"
					checked={popover.allDay}
					onCheckedChange={(allDay) => updatePopover({ allDay })}
				/>
			</div>

			{#if popover.allDay}
				<Popover.Root bind:open={isRangePickerOpen} onOpenChange={openDateRangePicker}>
					<Popover.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="outline"
								aria-label={text.startDate}
								class="h-8 w-full justify-between px-2 text-sm font-normal"
							>
								{dateRangeLabel}
								<ChevronDownIcon class="size-3.5 opacity-50" />
							</Button>
						{/snippet}
					</Popover.Trigger>
					<Popover.Content class="w-auto overflow-hidden p-0" align="start">
						<RangeCalendar
							bind:value={pickedDateRange}
							onValueChange={changeDateRange}
							captionLayout="dropdown"
						/>
					</Popover.Content>
				</Popover.Root>
			{:else}
				<div class="flex gap-1">
					<Popover.Root bind:open={isStartDatePickerOpen}>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button
									{...props}
									variant="outline"
									aria-label={text.startDate}
									class="h-8 flex-1 justify-between px-2 text-sm font-normal"
								>
									{formatDateKey(popover.dateKey)}
									<ChevronDownIcon class="size-3.5 opacity-50" />
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content class="w-auto overflow-hidden p-0" align="start">
							<Calendar
								type="single"
								value={startCalendarDate}
								onValueChange={changeStartDate}
								captionLayout="dropdown"
							/>
						</Popover.Content>
					</Popover.Root>
					<Input
						type="time"
						aria-label={text.startTime}
						class="bg-background h-8 w-24 appearance-none px-2 text-sm tabular-nums [&::-webkit-calendar-picker-indicator]:hidden"
						value={popover.startTime}
						onchange={(event) => changeStart({ startTime: event.currentTarget.value })}
					/>
				</div>

				<div class="flex gap-1">
					<Popover.Root bind:open={isEndDatePickerOpen}>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button
									{...props}
									variant="outline"
									aria-label={text.endDate}
									class="h-8 flex-1 justify-between px-2 text-sm font-normal"
								>
									{formatDateKey(popover.endDateKey)}
									<ChevronDownIcon class="size-3.5 opacity-50" />
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content class="w-auto overflow-hidden p-0" align="start">
							<Calendar
								type="single"
								value={endCalendarDate}
								onValueChange={changeEndDate}
								captionLayout="dropdown"
							/>
						</Popover.Content>
					</Popover.Root>
					<Input
						type="time"
						aria-label={text.endTime}
						class="bg-background h-8 w-24 appearance-none px-2 text-sm tabular-nums [&::-webkit-calendar-picker-indicator]:hidden"
						value={popover.endTime}
						onchange={(event) => updatePopover({ endTime: event.currentTarget.value })}
					/>
				</div>
			{/if}

			<Input
				aria-label={text.location}
				placeholder={text.location}
				autocomplete="off"
				class="h-8 px-2 text-sm"
				value={popover.location}
				oninput={(event) => updatePopover({ location: event.currentTarget.value })}
			/>

			<Input
				aria-label={text.description}
				placeholder={text.description}
				autocomplete="off"
				class="h-8 px-2 text-sm"
				value={popover.description}
				oninput={(event) => updatePopover({ description: event.currentTarget.value })}
			/>

			<CalendarParticipantSelector
				participants={popover.participants}
				candidates={participantCandidates}
				label={text.participants}
				placeholder={text.participantsPlaceholder}
				emptyText={text.participantsEmpty}
				summaryTemplate={text.participantsSummary}
				onChange={(participants) => updatePopover({ participants })}
			/>

			{#if calendarOptions.length > 1}
			<div class="flex">
				<Select.Root type="single" value={popover.calendarID} onValueChange={(calendarID) => updatePopover({ calendarID })}>
					<Select.Trigger size="sm" aria-label={text.calendar} class="h-8 flex-1 text-sm">
						{selectedCalendarName}
					</Select.Trigger>
					<Select.Content>
						{#each calendarOptions as option (option.id)}
							<Select.Item value={option.id} label={option.name}>{option.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			{/if}
		</div>

		{#if popover.mode === 'edit'}
			<footer class="border-border/50 flex border-t px-2 py-1.5">
				<Button
					variant="ghost"
					size="sm"
					class="text-destructive hover:text-destructive h-7 px-2"
					onclick={deletePopover}
				>
					{text.delete}
				</Button>
			</footer>
		{/if}
	</Popover.Content>
</Popover.Root>

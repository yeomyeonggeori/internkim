<script lang="ts">
	import AlignLeftIcon from '@lucide/svelte/icons/align-left';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import TrashIcon from '@lucide/svelte/icons/trash';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as InputGroup from '$lib/components/ui/input-group';
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

	function saveOnEnter(event: KeyboardEvent): void {
		if (event.key !== 'Enter' || !canSavePopover) return;
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
		aria-label={dialogLabel}
		class="max-h-[min(30rem,var(--bits-popover-content-available-height))] w-80 gap-0 overflow-y-auto p-0"
	>
		<div class="border-border/50 border-b">
			<Input
				value={popover.title}
				aria-label={text.title}
				placeholder={text.title}
				autocomplete="off"
				class="h-10 rounded-none border-0 px-3 text-sm font-semibold shadow-none focus-visible:ring-0"
				oninput={(event) => updatePopover({ title: event.currentTarget.value })}
				onkeydown={saveOnEnter}
			/>
		</div>

		<div class="grid gap-1.5 p-2">
			<div class="flex items-center gap-2 pl-1">
				<ClockIcon class="text-muted-foreground size-4 shrink-0" />
				<Label for="draft-all-day" class="text-muted-foreground text-xs font-normal">{text.allDay}</Label>
				<Switch
					id="draft-all-day"
					class="ml-auto"
					checked={popover.allDay}
					onCheckedChange={(allDay) => updatePopover({ allDay })}
				/>
			</div>

			<div class="flex gap-1 pl-7">
				<Input
					type="date"
					aria-label={text.startDate}
					class="h-8 flex-1 px-2 text-xs tabular-nums"
					value={popover.dateKey}
					onchange={(event) => changeStart({ startDateKey: event.currentTarget.value })}
				/>
				{#if !popover.allDay}
					<Input
						type="time"
						aria-label={text.startTime}
						class="h-8 w-24 px-2 text-xs tabular-nums"
						value={popover.startTime}
						onchange={(event) => changeStart({ startTime: event.currentTarget.value })}
					/>
				{/if}
			</div>

			<div class="flex gap-1 pl-7">
				<Input
					type="date"
					aria-label={text.endDate}
					class="h-8 flex-1 px-2 text-xs tabular-nums"
					value={popover.endDateKey}
					onchange={(event) => updatePopover({ endDateKey: event.currentTarget.value })}
				/>
				{#if !popover.allDay}
					<Input
						type="time"
						aria-label={text.endTime}
						class="h-8 w-24 px-2 text-xs tabular-nums"
						value={popover.endTime}
						onchange={(event) => updatePopover({ endTime: event.currentTarget.value })}
					/>
				{/if}
			</div>

			<InputGroup.Root class="h-8">
				<InputGroup.Addon>
					<MapPinIcon />
				</InputGroup.Addon>
				<InputGroup.Input
					aria-label={text.location}
					placeholder={text.location}
					autocomplete="off"
					class="text-xs"
					value={popover.location}
					oninput={(event) => updatePopover({ location: event.currentTarget.value })}
				/>
			</InputGroup.Root>

			<InputGroup.Root class="h-8">
				<InputGroup.Addon>
					<AlignLeftIcon />
				</InputGroup.Addon>
				<InputGroup.Input
					aria-label={text.description}
					placeholder={text.description}
					autocomplete="off"
					class="text-xs"
					value={popover.description}
					oninput={(event) => updatePopover({ description: event.currentTarget.value })}
				/>
			</InputGroup.Root>

			<div class="flex items-start gap-2 pl-1">
				<UsersIcon class="text-muted-foreground mt-1.5 size-4 shrink-0" />
				<CalendarParticipantSelector
					participants={popover.participants}
					candidates={participantCandidates}
					label={text.participants}
					placeholder={text.participantsPlaceholder}
					removeLabel={text.removeParticipantAction}
					onChange={(participants) => updatePopover({ participants })}
				/>
			</div>

			{#if calendarOptions.length > 1}
			<div class="flex items-center gap-2 pl-1">
				<CalendarIcon class="text-muted-foreground size-4 shrink-0" />
				<Select.Root type="single" value={popover.calendarID} onValueChange={(calendarID) => updatePopover({ calendarID })}>
					<Select.Trigger size="sm" aria-label={text.calendar} class="h-8 flex-1 text-xs">
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
					<TrashIcon />
					{text.delete}
				</Button>
			</footer>
		{/if}
	</Popover.Content>
</Popover.Root>

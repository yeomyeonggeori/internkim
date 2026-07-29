<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Popover from '$lib/components/ui/popover';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { CalendarAuditRow } from './calendar-audit';
	import { calendarDateTimeRangeChangesForStart } from './calendar-date-time-range';
	import { isDraftPopoverValid, type DraftPopoverState } from './calendar-draft-popover-state';
	import type { DraftPopoverText } from './calendar-draft-popover-text';
	import CalendarEventAuditCard from './calendar-event-audit-card.svelte';
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
		dialogLabel: string;
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
		participantCandidates,
		auditRows,
		auditLabel,
		dialogLabel,
		text,
		isSaving,
		updatePopover,
		savePopover,
		cancelPopover,
		deletePopover
	}: Props = $props();

	const anchorElement = $derived({
		getBoundingClientRect: () => {
			const originElement = popover.anchor?.originElement;
			if (originElement?.isConnected) return originElement.getBoundingClientRect();
			return new DOMRect(
				popover.anchor?.clientX ?? window.innerWidth / 2,
				popover.anchor?.clientY ?? window.innerHeight / 3,
				1,
				1
			);
		}
	});
	const canSavePopover = $derived(isDraftPopoverValid(popover));
	const selectedCalendarName = $derived(
		calendarOptions.find((option) => option.id === popover.calendarID)?.name ?? popover.calendarID
	);
	const auditEmptyText = $derived(popover.mode === 'edit' ? text.auditEmpty : '');

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
		side={popover.anchor?.originElement ? 'bottom' : 'right'}
		align={popover.anchor?.originElement ? 'end' : 'start'}
		collisionPadding={12}
		interactOutsideBehavior="ignore"
		aria-label={dialogLabel}
		class="max-h-[min(34rem,80svh)] w-96 gap-0 overflow-y-auto p-0"
	>
		<div class="bg-popover sticky top-0 z-10 border-b p-3">
			<Input
				value={popover.title}
				aria-label={text.title}
				placeholder={text.title}
				autocomplete="off"
				class="h-9 border-0 px-0 text-base font-semibold shadow-none focus-visible:ring-0"
				oninput={(event) => updatePopover({ title: event.currentTarget.value })}
				onkeydown={saveOnEnter}
			/>
		</div>

		<Field.Group class="p-3">
			<Field.Field orientation="horizontal">
				<Field.Label for="draft-all-day">{text.allDay}</Field.Label>
				<Switch id="draft-all-day" checked={popover.allDay} onCheckedChange={(allDay) => updatePopover({ allDay })} />
			</Field.Field>

			<Field.Field>
				<Field.Label for="draft-start-date">{text.startDate}</Field.Label>
				<div class="flex gap-2">
					<Input
						id="draft-start-date"
						type="date"
						class="flex-1 tabular-nums"
						value={popover.dateKey}
						onchange={(event) => changeStart({ startDateKey: event.currentTarget.value })}
					/>
					{#if !popover.allDay}
						<Input
							type="time"
							aria-label={text.startTime}
							class="w-28 tabular-nums"
							value={popover.startTime}
							onchange={(event) => changeStart({ startTime: event.currentTarget.value })}
						/>
					{/if}
				</div>
			</Field.Field>

			<Field.Field>
				<Field.Label for="draft-end-date">{text.endDate}</Field.Label>
				<div class="flex gap-2">
					<Input
						id="draft-end-date"
						type="date"
						class="flex-1 tabular-nums"
						value={popover.endDateKey}
						onchange={(event) => updatePopover({ endDateKey: event.currentTarget.value })}
					/>
					{#if !popover.allDay}
						<Input
							type="time"
							aria-label={text.endTime}
							class="w-28 tabular-nums"
							value={popover.endTime}
							onchange={(event) => updatePopover({ endTime: event.currentTarget.value })}
						/>
					{/if}
				</div>
			</Field.Field>

			<Field.Field>
				<Field.Label for="draft-location">{text.location}</Field.Label>
				<Input
					id="draft-location"
					value={popover.location}
					autocomplete="off"
					oninput={(event) => updatePopover({ location: event.currentTarget.value })}
				/>
			</Field.Field>

			<Field.Field>
				<Field.Label for="draft-description">{text.description}</Field.Label>
				<Textarea
					id="draft-description"
					rows={3}
					value={popover.description}
					oninput={(event) => updatePopover({ description: event.currentTarget.value })}
				/>
			</Field.Field>

			<Field.Field>
				<Field.Label>{text.participants}</Field.Label>
				<CalendarParticipantSelector
					participants={popover.participants}
					candidates={participantCandidates}
					label={text.participants}
					placeholder={text.participantsPlaceholder}
					removeLabel={text.removeParticipantAction}
					onChange={(participants) => updatePopover({ participants })}
				/>
			</Field.Field>

			<Field.Field>
				<Field.Label for="draft-calendar">{text.calendar}</Field.Label>
				<Select.Root type="single" value={popover.calendarID} onValueChange={(calendarID) => updatePopover({ calendarID })}>
					<Select.Trigger id="draft-calendar" class="w-full">{selectedCalendarName}</Select.Trigger>
					<Select.Content>
						{#each calendarOptions as option (option.id)}
							<Select.Item value={option.id} label={option.name}>{option.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</Field.Field>

			<CalendarEventAuditCard rows={auditRows} label={auditLabel} emptyText={auditEmptyText} />
		</Field.Group>

		{#if popover.mode === 'edit'}
			<footer class="bg-popover sticky bottom-0 flex items-center border-t p-3">
				<Button variant="ghost" size="sm" class="text-destructive hover:text-destructive" disabled={isSaving} onclick={deletePopover}>
					{text.delete}
				</Button>
			</footer>
		{/if}
	</Popover.Content>
</Popover.Root>

<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { cn } from '$lib/utils';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Popover from '$lib/components/ui/popover';
	import {
		calendarParticipantKey,
		calendarParticipantOptionLabel,
		type CalendarParticipant
	} from './calendar-participants';

	type Props = {
		participants: CalendarParticipant[];
		candidates: CalendarParticipant[];
		label: string;
		placeholder: string;
		emptyText: string;
		summaryTemplate: string;
		onChange?: (participants: CalendarParticipant[]) => void;
		disabled?: boolean;
	};

	let {
		participants = $bindable<CalendarParticipant[]>(),
		candidates,
		label,
		placeholder,
		emptyText,
		summaryTemplate,
		onChange,
		disabled = false
	}: Props = $props();

	let isPickerOpen = $state(false);

	const selectedKeys = $derived(new Set(participants.map(calendarParticipantKey)));
	const selectedSummary = $derived(participantSummaryLabel(participants));

	function participantSummaryLabel(selectedParticipants: CalendarParticipant[]): string {
		const [firstParticipant, ...otherParticipants] = selectedParticipants;
		if (!firstParticipant) return '';
		if (otherParticipants.length === 0) return firstParticipant.name;
		return summaryTemplate.replace('{name}', firstParticipant.name).replace('{count}', String(otherParticipants.length));
	}
	const canAddParticipants = $derived(!disabled && candidates.length > 0);

	function toggleParticipant(participant: CalendarParticipant): void {
		if (disabled) return;
		const participantKey = calendarParticipantKey(participant);
		const nextParticipants = selectedKeys.has(participantKey)
			? participants.filter((value) => calendarParticipantKey(value) !== participantKey)
			: [...participants, participant];
		participants = nextParticipants;
		onChange?.(nextParticipants);
	}
</script>

<Popover.Root bind:open={isPickerOpen}>
	<Popover.Trigger disabled={!canAddParticipants}>
		{#snippet child({ props })}
			<Button
				{...props}
				variant="outline"
				role="combobox"
				aria-expanded={isPickerOpen}
				aria-label={label}
				disabled={!canAddParticipants}
				class="h-8 w-full justify-between px-2 text-sm font-normal"
			>
				<span class="flex min-w-0 items-center gap-1.5">
					{#if participants.length > 0}
						<span class="flex shrink-0 items-center -space-x-1">
							{#each participants.slice(0, 3) as participant (calendarParticipantKey(participant))}
								<PersonAvatar
									name={participant.name}
									email={participant.email ?? ''}
									seed={calendarParticipantKey(participant)}
									image={participant.image ?? ''}
									class="ring-background size-4 ring-2"
								/>
							{/each}
						</span>
					{/if}
					<span class={cn('truncate', participants.length === 0 && 'text-muted-foreground')}>
						{selectedSummary || label}
					</span>
				</span>
				<ChevronsUpDownIcon class="size-3.5 shrink-0 opacity-50" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-56 p-0" align="start">
		<Command.Root>
			<Command.Input {placeholder} class="h-9" />
			<Command.List>
				<Command.Empty class="py-4 text-sm">{emptyText}</Command.Empty>
				{#each candidates as candidate (calendarParticipantKey(candidate))}
					<Command.Item
						value={calendarParticipantOptionLabel(candidate, candidates)}
						onSelect={() => toggleParticipant(candidate)}
					>
						<PersonAvatar
							name={candidate.name}
							email={candidate.email ?? ''}
							seed={calendarParticipantKey(candidate)}
							image={candidate.image ?? ''}
							class="size-5"
						/>
						<span class="truncate">{calendarParticipantOptionLabel(candidate, candidates)}</span>
						{#if selectedKeys.has(calendarParticipantKey(candidate))}
							<CheckIcon class="ml-auto size-4" />
						{/if}
					</Command.Item>
				{/each}
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>

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
		onChange?: (participants: CalendarParticipant[]) => void;
		disabled?: boolean;
	};

	let {
		participants = $bindable<CalendarParticipant[]>(),
		candidates,
		label,
		placeholder,
		emptyText,
		onChange,
		disabled = false
	}: Props = $props();

	let isPickerOpen = $state(false);

	const selectedKeys = $derived(new Set(participants.map(calendarParticipantKey)));
	const selectedNames = $derived(participants.map((participant) => participant.name).join(', '));
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
				class="h-8 w-full justify-between px-2 text-xs font-normal"
			>
				<span class={cn('truncate', !selectedNames && 'text-muted-foreground')}>{selectedNames || label}</span>
				<ChevronsUpDownIcon class="size-3.5 shrink-0 opacity-50" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-56 p-0" align="start">
		<Command.Root>
			<Command.Input {placeholder} class="h-8 text-xs" />
			<Command.List>
				<Command.Empty class="py-4 text-xs">{emptyText}</Command.Empty>
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

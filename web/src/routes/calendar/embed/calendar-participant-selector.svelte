<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
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
		removeLabel: string;
		onChange?: (participants: CalendarParticipant[]) => void;
		disabled?: boolean;
	};

	let {
		participants = $bindable<CalendarParticipant[]>(),
		candidates,
		label,
		placeholder,
		emptyText,
		removeLabel,
		onChange,
		disabled = false
	}: Props = $props();

	let isPickerOpen = $state(false);

	const selectedKeys = $derived(new Set(participants.map(calendarParticipantKey)));
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

<div class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
	{#each participants as participant (calendarParticipantKey(participant))}
		<Badge variant="secondary" class="h-6 gap-1 py-0 pr-1 pl-1 text-xs font-normal">
			<PersonAvatar
				name={participant.name}
				email={participant.email ?? ''}
				seed={calendarParticipantKey(participant)}
				image={participant.image ?? ''}
				class="size-4"
			/>
			<span class="max-w-24 truncate">{participant.name}</span>
			<button
				type="button"
				class="hover:text-foreground text-muted-foreground rounded-full"
				aria-label={removeLabel.replace('{name}', participant.name)}
				{disabled}
				onclick={() => toggleParticipant(participant)}
			>
				<XIcon class="size-3" />
			</button>
		</Badge>
	{/each}

	<Popover.Root bind:open={isPickerOpen}>
		<Popover.Trigger disabled={!canAddParticipants}>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="ghost"
					size="sm"
					class="text-muted-foreground h-6 gap-1 px-1.5 text-xs"
					disabled={!canAddParticipants}
				>
					<PlusIcon class="size-3.5" />
					{label}
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
</div>

<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import XIcon from '@lucide/svelte/icons/x';
	import {
		calendarParticipantKey,
		calendarParticipantMatchesSearch,
		calendarParticipantOptionLabel,
		type CalendarParticipant
	} from './calendar-participants';

	type Props = {
		participants: CalendarParticipant[];
		candidates: CalendarParticipant[];
		label: string;
		placeholder: string;
		removeLabel: string;
		onChange?: (participants: CalendarParticipant[]) => void;
		disabled?: boolean;
	};

	let {
		participants = $bindable<CalendarParticipant[]>(),
		candidates,
		label,
		placeholder,
		removeLabel,
		onChange,
		disabled = false
	}: Props = $props();

	let inputValue = $state('');
	let inputFocused = $state(false);
	let highlightedIndex = $state(0);
	let blurTimer: ReturnType<typeof setTimeout> | null = null;
	let listboxID = $props.id();

	const selectedKeys = $derived(new Set(participants.map(calendarParticipantKey)));
	const filteredCandidates = $derived(
		candidates
			.filter((candidate) => !selectedKeys.has(calendarParticipantKey(candidate)))
			.filter((candidate) => calendarParticipantMatchesSearch(candidate, inputValue))
	);
	const showSuggestions = $derived(!disabled && inputFocused && filteredCandidates.length > 0);
	const canAddParticipants = $derived(!disabled && candidates.length > 0);

	function addParticipant(participant: CalendarParticipant): void {
		if (disabled) return;
		const nextParticipants = [...participants, participant];
		participants = nextParticipants;
		onChange?.(nextParticipants);
		inputValue = '';
		highlightedIndex = 0;
	}

	function removeParticipant(participant: CalendarParticipant): void {
		if (disabled) return;
		const removedKey = calendarParticipantKey(participant);
		const nextParticipants = participants.filter((value) => calendarParticipantKey(value) !== removedKey);
		participants = nextParticipants;
		onChange?.(nextParticipants);
	}

	function handleInputFocus(): void {
		if (blurTimer) clearTimeout(blurTimer);
		inputFocused = true;
	}

	function handleInputBlur(): void {
		blurTimer = setTimeout(() => {
			inputFocused = false;
			highlightedIndex = 0;
		}, 120);
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			inputFocused = false;
			highlightedIndex = 0;
			return;
		}
		if (!showSuggestions) return;
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			highlightedIndex = (highlightedIndex + 1) % filteredCandidates.length;
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			highlightedIndex = (highlightedIndex - 1 + filteredCandidates.length) % filteredCandidates.length;
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			addParticipant(filteredCandidates[highlightedIndex] ?? filteredCandidates[0]);
		}
	}
</script>

<div class="calendar-participant-selector">
	<div class="calendar-participant-combobox">
		<div class="calendar-participant-input-shell">
			{#each participants as participant (participant.personID || participant.email || participant.name)}
				<Badge variant="outline" class="calendar-participant-chip">
					<PersonAvatar name={participant.name} email={participant.email ?? ''} seed={participant.personID || participant.email || participant.name} image={participant.image ?? ''} class="size-4" />
					<span>{participant.name}</span>
					<button
						type="button"
						class="calendar-participant-remove"
						aria-label={removeLabel.replace('{name}', participant.name)}
						disabled={disabled}
						onclick={() => removeParticipant(participant)}
					>
						<XIcon class="size-3" />
					</button>
				</Badge>
			{/each}
			<input
				bind:value={inputValue}
				{placeholder}
				disabled={!canAddParticipants}
				aria-label={label}
				role="combobox"
				aria-expanded={showSuggestions}
				aria-autocomplete="list"
				aria-controls={showSuggestions ? listboxID : undefined}
				aria-activedescendant={showSuggestions ? `${listboxID}-${highlightedIndex}` : undefined}
				onfocus={handleInputFocus}
				onblur={handleInputBlur}
				onkeydown={handleKeydown}
			/>
		</div>
		{#if showSuggestions}
			<div id={listboxID} role="listbox" class="calendar-participant-listbox">
				{#each filteredCandidates as candidate, index (calendarParticipantKey(candidate))}
					<button
						id="{listboxID}-{index}"
						type="button"
						role="option"
						aria-selected={index === highlightedIndex}
						class="calendar-participant-option"
						onpointerdown={(event) => {
							event.preventDefault();
							addParticipant(candidate);
						}}
						onclick={(event) => {
							if (event.detail === 0) addParticipant(candidate);
						}}
					>
						<PersonAvatar name={candidate.name} email={candidate.email ?? ''} seed={candidate.personID || candidate.email || candidate.name} image={candidate.image ?? ''} class="size-5" />
						<span class="calendar-participant-option-text">
							<span>{calendarParticipantOptionLabel(candidate, candidates)}</span>
						</span>
					</button>
				{/each}
			</div>
		{/if}
	</div>
</div>

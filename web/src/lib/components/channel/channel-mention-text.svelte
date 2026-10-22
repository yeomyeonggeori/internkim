<script lang="ts">
	import { getContext } from 'svelte';
	import MentionChip from './mention-chip.svelte';
	import MentionPersonCard from './mention-person-card.svelte';
	import { mentionLabelsContext, mentionPieces, type MentionLabel } from '$lib/messenger/mention-text';
	import { personProfileContext, type PersonProfileActions } from '$lib/messenger/person-profile';

	let { text = '' }: { text?: string } = $props();

	const labels = getContext<() => MentionLabel[]>(mentionLabelsContext);
	const profile = getContext<PersonProfileActions | undefined>(personProfileContext);
	const pieces = $derived(mentionPieces(text, labels?.() ?? []));
</script>

{#each pieces as piece, index (index)}{#if piece.isMention && piece.externalID && profile}<MentionPersonCard
			text={piece.text}
			person={{ externalID: piece.externalID, name: piece.text.slice(1) }}
			actions={profile}
		/>{:else if piece.isMention}<MentionChip text={piece.text} />{:else}{piece.text}{/if}{/each}

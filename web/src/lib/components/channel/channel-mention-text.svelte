<script lang="ts">
	import { getContext } from 'svelte';
	import { mentionLabelsContext, mentionPieces } from '$lib/messenger/mention-text';

	let { text = '' }: { text?: string } = $props();

	const labels = getContext<() => string[]>(mentionLabelsContext);
	const pieces = $derived(mentionPieces(text, labels?.() ?? []));
</script>

{#each pieces as piece, index (index)}{#if piece.isMention}<span
			class="bg-primary/15 text-primary rounded px-1 font-medium">{piece.text}</span
		>{:else}{piece.text}{/if}{/each}

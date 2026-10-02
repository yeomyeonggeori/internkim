<script lang="ts">
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { reactionPeopleLabel } from './channel-reactions';
	import type { ChannelMessageReaction } from './channel-api';
	import { customEmoji } from '$lib/stores/custom-emoji.svelte';

	let {
		reactions,
		canChange,
		side,
		farCorner,
		nameWidthPixels,
		onToggle
	}: {
		reactions: ChannelMessageReaction[];
		canChange: boolean;
		side: 'top' | 'bottom';
		farCorner: 'start' | 'end';
		nameWidthPixels: number;
		onToggle: (reaction: ChannelMessageReaction) => void;
	} = $props();

	const text = createPageText(channelText);
	const cornerInsetPixels = 12;
	const gapAfterNamePixels = 8;

	let pill = $state<HTMLDivElement | null>(null);
	let pillWidthPixels = $state(0);
	let bubbleWidthPixels = $state(0);

	const placement = $derived.by(() => {
		if (pillWidthPixels === 0) return undefined;
		const fromFarCorner = bubbleWidthPixels - cornerInsetPixels - pillWidthPixels;
		if (farCorner === 'start') return `right: ${Math.max(cornerInsetPixels, fromFarCorner)}px; left: auto`;
		const clearOfName = nameWidthPixels > 0 ? nameWidthPixels + gapAfterNamePixels : cornerInsetPixels;
		return `left: ${Math.max(clearOfName, fromFarCorner)}px; right: auto`;
	});

	$effect(() => {
		const bubble = pill?.parentElement;
		if (!pill || !bubble) return;
		const measured = pill;
		const measure = (): void => {
			pillWidthPixels = measured.offsetWidth;
			bubbleWidthPixels = bubble.clientWidth;
		};
		const observer = new ResizeObserver(measure);
		observer.observe(measured);
		observer.observe(bubble);
		measure();
		return () => observer.disconnect();
	});

	function toggleWhenJoinable(reaction: ChannelMessageReaction): void {
		if (canChange && !reactionImage(reaction)) onToggle(reaction);
	}

	function reactionImage(reaction: ChannelMessageReaction): string | undefined {
		return reaction.imageURL ?? customEmoji.nameToURL.get(reaction.value);
	}
</script>

<Bubble.Reactions bind:ref={pill} {side} align={farCorner} style={placement}>
	{#each reactions as reaction (reaction.value)}
		{@const imageURL = reactionImage(reaction)}
		{@const peopleLabel = reactionPeopleLabel(
			(reaction.people ?? []).map((person) => person.name).filter(Boolean),
			{ reactedBy: text.reactedBy, reactedByMore: text.reactedByMore }
		)}
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant={reaction.reactedByMe ? 'outline' : 'ghost'}
						size="xs"
						class={reaction.reactedByMe ? 'border-foreground/30 hover:bg-foreground/10' : 'hover:bg-foreground/10'}
						aria-pressed={reaction.reactedByMe ?? false}
						onclick={() => toggleWhenJoinable(reaction)}
					>
						{#if imageURL}
							<img src={imageURL} alt={reaction.emoji} class="inline size-4" />
						{:else}
							{reaction.emoji}
						{/if}
						{#if reaction.count > 1}<span>{reaction.count}</span>{/if}
					</Button>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content>
				<div class="flex flex-col gap-0.5">
					<div class="flex items-center gap-1">
						{#if reaction.imageURL}
							<img src={reaction.imageURL} alt={reaction.emoji} class="inline size-4" />
						{:else}
							<span>{reaction.emoji}</span>
						{/if}
						{#if reaction.value !== reaction.emoji}
							<span>:{reaction.value}:</span>
						{/if}
					</div>
					{#if peopleLabel}
						<span>{peopleLabel}</span>
					{/if}
				</div>
			</Tooltip.Content>
		</Tooltip.Root>
	{/each}
</Bubble.Reactions>

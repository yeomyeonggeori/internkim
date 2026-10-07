<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { toggleVariants } from '$lib/components/ui/toggle/index.js';
	import { cn } from '$lib/utils.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { reactionPeopleLabel } from './channel-reactions';
	import type { ChannelMessageReaction } from './channel-api';
	import { customEmoji } from '$lib/stores/custom-emoji.svelte';
	import LoadingImage from '$lib/components/loading-image.svelte';
	import SmilePlusIcon from '@lucide/svelte/icons/smile-plus';

	let {
		reactions,
		canChange,
		onToggle,
		onAdd
	}: {
		reactions: ChannelMessageReaction[];
		canChange: boolean;
		onToggle: (reaction: ChannelMessageReaction) => void;
		onAdd: (anchor: HTMLElement) => void;
	} = $props();

	const text = createPageText(channelText);
	const chipClass = cn(toggleVariants({ variant: 'outline', size: 'sm' }), 'h-6 min-w-0 rounded-full px-2 tabular-nums aria-pressed:border-foreground/30');

	function toggleWhenJoinable(reaction: ChannelMessageReaction): void {
		if (canChange && !reactionImage(reaction)) onToggle(reaction);
	}

	function reactionImage(reaction: ChannelMessageReaction): string | undefined {
		return customEmoji.nameToURL.get(reaction.value) || reaction.imageURL;
	}
</script>

<div class="message-reactions flex flex-wrap gap-1.5 group-data-[align=end]/message:justify-end">
	{#each reactions as reaction (reaction.value)}
		{@const imageURL = reactionImage(reaction)}
		{@const shortcode = `:${reaction.value}:`}
		{@const peopleLabel = reactionPeopleLabel(
			(reaction.people ?? []).map((person) => person.name).filter(Boolean),
			{ reactedBy: text.reactedBy, reactedByMore: text.reactedByMore }
		)}
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<button
						{...props}
						type="button"
						data-slot="toggle"
						class={chipClass}
						aria-pressed={reaction.reactedByMe ?? false}
						aria-label={imageURL ? `${shortcode} ${reaction.count}` : undefined}
						onclick={() => toggleWhenJoinable(reaction)}
					>
						{#if imageURL}
							<LoadingImage src={imageURL} alt={shortcode} fallbackText={shortcode} fill loading="eager" class="size-4 shrink-0 rounded-none" />
						{:else}
							<span class="text-sm leading-none">{reaction.emoji}</span>
						{/if}
						<span>{reaction.count}</span>
					</button>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content>
				<div class="flex flex-col gap-0.5">
					<div class="flex items-center gap-1">
						{#if imageURL}
							<LoadingImage src={imageURL} alt={shortcode} fallbackText={shortcode} fill loading="eager" class="size-4 shrink-0 rounded-none" />
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
	{#if canChange}
		<button type="button" data-slot="toggle" class={cn(chipClass, 'text-muted-foreground')} aria-label={text.addReaction} onclick={(event) => onAdd(event.currentTarget)}>
			<SmilePlusIcon />
		</button>
	{/if}
</div>

<style>
	@media (max-width: 639px) {
		.message-reactions :global([data-slot='toggle']) {
			min-height: 24px;
		}
	}
</style>

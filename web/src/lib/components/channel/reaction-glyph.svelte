<script lang="ts" module>
	import { customEmoji } from '$lib/stores/custom-emoji.svelte';
	import type { ChannelMessageReaction } from './channel-api';

	export function reactionImage(reaction: ChannelMessageReaction): string | undefined {
		return customEmoji.nameToURL.get(reaction.value) || reaction.imageURL;
	}
</script>

<script lang="ts">
	import LoadingImage from '$lib/components/loading-image.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { cn } from '$lib/utils.js';

	let { reaction, class: className }: { reaction: ChannelMessageReaction; class?: string } = $props();

	const imageURL = $derived(reactionImage(reaction));
	const isImageDrawing = $derived(Boolean(reaction.imageURL) && customEmoji.isDrawing(reaction.value));
	const shortcode = $derived(`:${reaction.value}:`);
</script>

{#if isImageDrawing}
	<Skeleton class="size-4 shrink-0 rounded-none" />
{:else if imageURL}
	<LoadingImage src={imageURL} alt={shortcode} fallbackText={shortcode} fill loading="eager" class="size-4 shrink-0 rounded-none" />
{:else}
	<span class={cn('leading-none', className)}>{reaction.emoji}</span>
{/if}

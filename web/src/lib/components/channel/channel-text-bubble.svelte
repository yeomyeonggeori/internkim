<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import type { MentionLabel } from '$lib/messenger/mention-text';
	import ChannelMessageBody from './channel-message-body.svelte';

	let {
		mine,
		source,
		mentionLabels = [],
		children
	}: { mine: boolean; source: string; mentionLabels?: MentionLabel[]; children?: Snippet } = $props();
</script>

<Bubble.Root variant={mine ? 'default' : 'muted'} class="max-w-[min(80%,32rem)]">
	<Bubble.Content>
		<div class="chat-markdown prose prose-sm dark:prose-invert max-w-none">
			<ChannelMessageBody {source} {mentionLabels} />
		</div>
	</Bubble.Content>
	{@render children?.()}
</Bubble.Root>

<style>
	.chat-markdown {
		overflow-wrap: anywhere;
		word-break: keep-all;
	}
	.chat-markdown,
	.chat-markdown :global(*) {
		color: inherit;
	}
	.chat-markdown :global(p:first-child) {
		margin-top: 0;
	}
	.chat-markdown :global(p:last-child) {
		margin-bottom: 0;
	}
	.chat-markdown :global(blockquote) {
		font-style: normal;
	}
	.chat-markdown :global(blockquote p::before),
	.chat-markdown :global(blockquote p::after) {
		content: none;
	}
	.chat-markdown :global(table) {
		display: block;
		max-width: 100%;
		overflow-x: auto;
		border-collapse: collapse;
		margin: 0.5rem 0;
		font-size: 0.9em;
		border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
	}
	.chat-markdown :global(th),
	.chat-markdown :global(td) {
		border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
		padding: 0.375rem 0.625rem;
		text-align: left;
		white-space: nowrap;
	}
	.chat-markdown :global(thead th) {
		background: color-mix(in srgb, currentColor 6%, transparent);
		font-weight: 600;
	}
	.chat-markdown :global(tbody tr:nth-child(even)) {
		background: color-mix(in srgb, currentColor 6%, transparent);
	}
	.chat-markdown :global(img) {
		display: inline-block;
		height: 1.4em;
		width: auto;
		margin: 0;
		vertical-align: text-bottom;
	}
</style>

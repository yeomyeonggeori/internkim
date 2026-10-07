<script lang="ts">
	import * as Command from '$lib/components/ui/command/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { emojiCategories, searchEmoji } from '$lib/messenger/emoji-catalog';
	import { quickEmojiGlyphs, rememberEmojiGlyph } from '$lib/messenger/recent-emoji';
	import type { Snippet } from 'svelte';

	let {
		open = $bindable(false),
		onPick,
		trigger,
		customAnchor = null,
		side = 'top',
		align = 'end'
	}: {
		open?: boolean;
		onPick: (glyph: string) => void;
		trigger?: Snippet<[{ props: Record<string, unknown> }]>;
		customAnchor?: HTMLElement | null;
		side?: 'top' | 'bottom';
		align?: 'start' | 'center' | 'end';
	} = $props();

	const text = createPageText(channelText);

	let query = $state('');
	let content = $state<HTMLElement | null>(null);
	const quickGlyphs = $derived(open ? quickEmojiGlyphs(8) : []);
	const searchResults = $derived(query.trim() === '' ? [] : searchEmoji(query, 40));
	const categoryLabels = $derived<Record<string, string>>({
		'Smileys & Emotion': text.emojiSmileys,
		'People & Body': text.emojiPeople,
		'Animals & Nature': text.emojiNature,
		'Food & Drink': text.emojiFood,
		'Travel & Places': text.emojiTravel,
		Activities: text.emojiActivities,
		Objects: text.emojiObjects,
		Symbols: text.emojiSymbols,
		Flags: text.emojiFlags
	});

	$effect(() => {
		if (!open) return;
		const closeOnScrollElsewhere = (event: Event): void => {
			if (event.target instanceof Node && content?.contains(event.target)) return;
			open = false;
		};
		window.addEventListener('scroll', closeOnScrollElsewhere, { capture: true, passive: true });
		return () => window.removeEventListener('scroll', closeOnScrollElsewhere, { capture: true });
	});

	function pickEmoji(glyph: string): void {
		rememberEmojiGlyph(glyph);
		onPick(glyph);
		open = false;
		query = '';
	}
</script>

<Popover.Root bind:open>
	{#if trigger}
		<Popover.Trigger>
			{#snippet child({ props })}
				{@render trigger({ props })}
			{/snippet}
		</Popover.Trigger>
	{/if}
	<Popover.Content bind:ref={content} {side} {align} {customAnchor} class="w-80 max-w-[calc(100vw-1rem)] p-0">
		<Command.Root shouldFilter={false}>
			<Command.Input bind:value={query} placeholder={text.searchEmoji} />
			<Command.List>
				{#if query.trim() === ''}
					<Command.Group heading={text.quickEmojiTitle}>
						<div class="grid grid-cols-7 p-1 sm:grid-cols-8 max-sm:[&>:nth-child(8)]:hidden">
							{#each quickGlyphs as glyph (glyph)}
								<button
									type="button"
									aria-label={glyph}
									onclick={() => pickEmoji(glyph)}
									class="hover:bg-muted focus-visible:bg-muted flex aspect-square items-center justify-center rounded-md text-2xl leading-none outline-hidden"
								>
									{glyph}
								</button>
							{/each}
						</div>
					</Command.Group>
					{#each emojiCategories() as category (category.name)}
						<Command.Group heading={categoryLabels[category.name] ?? category.name}>
							<div class="grid grid-cols-7 p-1 sm:grid-cols-8 [contain-intrinsic-size:auto_12rem] [content-visibility:auto]">
								{#each category.emoji as emoji (emoji.name)}
									<button
										type="button"
										aria-label={emoji.name}
										title={emoji.name}
										onclick={() => pickEmoji(emoji.glyph)}
										class="hover:bg-muted focus-visible:bg-muted flex aspect-square items-center justify-center rounded-md text-2xl leading-none outline-hidden"
									>
										{emoji.glyph}
									</button>
								{/each}
							</div>
						</Command.Group>
					{/each}
				{:else}
					{#each searchResults as emoji (emoji.name)}
						<Command.Item value={emoji.name} class="max-sm:min-h-11" onSelect={() => pickEmoji(emoji.glyph)}>
							<span class="text-xl leading-none">{emoji.glyph}</span>
							<span class="truncate">{emoji.name}</span>
						</Command.Item>
					{/each}
					<Command.Empty>{text.noEmojiFound}</Command.Empty>
				{/if}
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>

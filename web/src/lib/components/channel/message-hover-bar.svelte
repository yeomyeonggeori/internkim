<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { quickEmojiGlyphs, rememberEmojiGlyph } from '$lib/messenger/recent-emoji';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import SmilePlusIcon from '@lucide/svelte/icons/smile-plus';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';

	let {
		mine,
		canReact,
		canReply,
		onReact,
		onOpenPicker,
		onReply,
		onOpenMenu
	}: {
		mine: boolean;
		canReact: boolean;
		canReply: boolean;
		onReact: (glyph: string) => void;
		onOpenPicker: (anchor: HTMLElement) => void;
		onReply: () => void;
		onOpenMenu: (anchor: HTMLElement) => void;
	} = $props();

	const text = createPageText(channelText);

	function reactWith(glyph: string): void {
		rememberEmojiGlyph(glyph);
		onReact(glyph);
	}
</script>

<div
	data-slot="message-hover-bar"
	class={[
		'frosted-surface invisible absolute -top-4 z-20 hidden items-center gap-0.5 rounded-full border p-0.5 opacity-0 shadow-md transition-[opacity,visibility] delay-150 duration-100',
		'[@media(hover:hover)]:flex [@media(hover:hover)]:group-hover/row:visible [@media(hover:hover)]:group-hover/row:opacity-100 [@media(hover:hover)]:group-hover/row:delay-0',
		mine ? 'left-4' : 'right-4'
	]}
>
	{#if canReact}
		{#each quickEmojiGlyphs(5) as glyph (glyph)}
			<Button variant="ghost" size="icon-sm" class="rounded-full text-base" aria-label={glyph} onclick={() => reactWith(glyph)}>
				{glyph}
			</Button>
		{/each}
		<div class="bg-foreground/10 mx-0.5 h-4 w-px"></div>
		<Button variant="ghost" size="icon-sm" class="text-muted-foreground rounded-full" aria-label={text.addReaction} onclick={(event) => onOpenPicker(event.currentTarget)}>
			<SmilePlusIcon />
		</Button>
	{/if}
	{#if canReply}
		<Button variant="ghost" size="icon-sm" class="text-muted-foreground rounded-full" aria-label={text.reply} onclick={onReply}>
			<ReplyIcon />
		</Button>
	{/if}
	<Button variant="ghost" size="icon-sm" class="text-muted-foreground rounded-full" aria-label={text.messageActions} onclick={(event) => onOpenMenu(event.currentTarget)}>
		<EllipsisVerticalIcon />
	</Button>
</div>

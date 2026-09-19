<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { quickEmojiGlyphs, rememberEmojiGlyph } from '$lib/messenger/recent-emoji';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import type { MessageCopy } from './message-copy';
	import type { Snippet } from 'svelte';

	let {
		canChange,
		canReply,
		canDelete,
		copyKind,
		open = $bindable(false),
		addReaction,
		onQuickReact,
		onReply,
		onCopy,
		onDelete
	}: {
		canChange: boolean;
		canReply: boolean;
		canDelete: boolean;
		copyKind: MessageCopy['kind'];
		open?: boolean;
		addReaction?: Snippet;
		onQuickReact: (glyph: string) => void;
		onReply: () => void;
		onCopy: () => void;
		onDelete: () => void;
	} = $props();

	const text = createPageText(channelText);

	function reactWithQuickEmoji(glyph: string): void {
		rememberEmojiGlyph(glyph);
		onQuickReact(glyph);
	}
</script>

<div role="toolbar" aria-label={text.messageActions} class="flex items-center gap-px">
	{#if canChange}
		{#each quickEmojiGlyphs(3) as glyph (glyph)}
			<Button variant="ghost" size="icon-xs" aria-label={glyph} onclick={() => reactWithQuickEmoji(glyph)}>
				{glyph}
			</Button>
		{/each}
		{@render addReaction?.()}
		<Separator orientation="vertical" class="mx-0.5 !h-4" />
	{/if}
	{#if canReply}
		<Button variant="ghost" size="icon-xs" aria-label={text.reply} onclick={onReply}>
			<ReplyIcon />
		</Button>
	{/if}
	{#if copyKind !== 'nothing'}
		<Button
			variant="ghost"
			size="icon-xs"
			aria-label={copyKind === 'picture' ? text.copyPicture : text.copyMessage}
			onclick={onCopy}
		>
			<CopyIcon />
		</Button>
	{/if}
	{#if canChange && canDelete}
		<DropdownMenu.Root bind:open>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button {...props} variant="ghost" size="icon-xs" aria-label={text.moreActions}>
						<EllipsisVerticalIcon />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end">
				<DropdownMenu.Item variant="destructive" onSelect={onDelete}>
					<Trash2Icon />
					<span>{text.delete}</span>
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	{/if}
</div>

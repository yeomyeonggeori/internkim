<script lang="ts">
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { quickEmojiGlyphs, rememberEmojiGlyph } from '$lib/messenger/recent-emoji';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import ImageIcon from '@lucide/svelte/icons/image';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import SmilePlusIcon from '@lucide/svelte/icons/smile-plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import type { Snippet } from 'svelte';

	let {
		canChange,
		canReply,
		canDelete,
		canEdit,
		canCopyText,
		canCopyPicture,
		canDownloadPicture,
		disabled,
		open = $bindable(false),
		onQuickReact,
		onOpenPicker,
		onReply,
		onEdit,
		onCopyText,
		onCopyPicture,
		onDownloadPicture,
		onDelete,
		children
	}: {
		canChange: boolean;
		canReply: boolean;
		canDelete: boolean;
		canEdit: boolean;
		canCopyText: boolean;
		canCopyPicture: boolean;
		canDownloadPicture: boolean;
		disabled?: boolean;
		open?: boolean;
		onQuickReact: (glyph: string) => void;
		onOpenPicker: () => void;
		onReply: () => void;
		onEdit: () => void;
		onCopyText: () => void;
		onCopyPicture: () => void;
		onDownloadPicture: () => void;
		onDelete: () => void;
		children: Snippet;
	} = $props();

	const text = createPageText(channelText);

	function reactWithQuickEmoji(glyph: string): void {
		rememberEmojiGlyph(glyph);
		onQuickReact(glyph);
	}
</script>

<ContextMenu.Root bind:open>
	<ContextMenu.Trigger {disabled} class="select-text [@media(hover:none)]:select-none [@media(hover:none)]:[-webkit-touch-callout:none]">
		{@render children()}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52">
		{#if canChange}
			<div class="flex items-center justify-between px-1 py-1">
				{#each quickEmojiGlyphs(5) as glyph (glyph)}
					<ContextMenu.Item
						class="size-8 justify-center rounded-full p-0 text-lg"
						aria-label={glyph}
						onSelect={() => reactWithQuickEmoji(glyph)}
					>
						{glyph}
					</ContextMenu.Item>
				{/each}
			</div>
			<ContextMenu.Item onSelect={onOpenPicker}>
				<SmilePlusIcon />
				<span>{text.addReaction}</span>
			</ContextMenu.Item>
			<ContextMenu.Separator />
		{/if}
		{#if canReply}
			<ContextMenu.Item onSelect={onReply}>
				<ReplyIcon />
				<span>{text.reply}</span>
			</ContextMenu.Item>
		{/if}
		{#if canChange && canEdit}
			<ContextMenu.Item onSelect={onEdit}>
				<PencilIcon />
				<span>{text.editMessage}</span>
			</ContextMenu.Item>
		{/if}
		{#if canCopyPicture}
			<ContextMenu.Item onSelect={onCopyPicture}>
				<ImageIcon />
				<span>{text.copyPicture}</span>
			</ContextMenu.Item>
		{/if}
		{#if canDownloadPicture}
			<ContextMenu.Item onSelect={onDownloadPicture}>
				<DownloadIcon />
				<span>{text.downloadPicture}</span>
			</ContextMenu.Item>
		{/if}
		{#if canCopyText}
			<ContextMenu.Item onSelect={onCopyText}>
				<CopyIcon />
				<span>{text.copyMessage}</span>
			</ContextMenu.Item>
		{/if}
		{#if canChange && canDelete}
			<ContextMenu.Separator />
			<ContextMenu.Item variant="destructive" onSelect={onDelete}>
				<Trash2Icon />
				<span>{text.delete}</span>
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>

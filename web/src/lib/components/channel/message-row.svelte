<script lang="ts">
	import * as Message from '$lib/components/ui/message/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import EmojiPicker from './emoji-picker.svelte';
	import MessageContextMenu from './message-context-menu.svelte';
	import MessageToolbar from './message-toolbar.svelte';
	import { swallowClickAfterTouchHold } from './swallow-click-after-touch-hold';
	import { swipeToReply } from './swipe-to-reply';
	import type { ChannelMessage } from './channel-api';
	import type { MessageCopy } from './message-copy';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import SmilePlusIcon from '@lucide/svelte/icons/smile-plus';
	import { tick, type Snippet } from 'svelte';

	let {
		message,
		mine,
		startsGroup,
		endsGroup,
		senderName,
		canChange,
		canReply,
		canDelete,
		isSettled,
		hasFooter,
		copyable,
		onReply,
		onCopy,
		onDelete,
		onReact,
		avatar,
		children,
		footer
	}: {
		message: ChannelMessage;
		mine: boolean;
		startsGroup: boolean;
		endsGroup: boolean;
		senderName: string;
		canChange: boolean;
		canReply: boolean;
		canDelete: boolean;
		isSettled: boolean;
		hasFooter: boolean;
		copyable: MessageCopy;
		onReply: () => void;
		onCopy: (wanted: MessageCopy) => void;
		onDelete: () => void;
		onReact: (glyph: string) => void;
		avatar: Snippet;
		children: Snippet<[{ nameWidthPixels: number; hasFooter: boolean; isToolbarShown: boolean; toolbar: Snippet }]>;
		footer: Snippet;
	} = $props();

	const text = createPageText(channelText);

	let contentElement = $state<HTMLDivElement | null>(null);
	let measuredNameWidthPixels = $state(0);
	let isPointerInside = $state(false);
	let isToolbarMenuOpen = $state(false);
	let isToolbarPickerOpen = $state(false);
	let isAnchoredPickerOpen = $state(false);
	let isContextMenuOpen = $state(false);
	let pictureUnderPointer = $state('');

	const canChangeThis = $derived(canChange && isSettled);
	const hasActions = $derived(canChangeThis || canReply || copyable.kind !== 'nothing');
	const pictureToCopy = $derived(pictureUnderPointer || (copyable.kind === 'picture' ? copyable.address : ''));
	const hasHeader = $derived(startsGroup && !mine && senderName !== '');
	const isToolbarShown = $derived(
		isSettled && hasActions && (isPointerInside || isToolbarMenuOpen || isToolbarPickerOpen)
	);
	const isHeld = $derived(isToolbarMenuOpen || isToolbarPickerOpen || isAnchoredPickerOpen);

	function notePointerInside(event: PointerEvent): void {
		if (event.pointerType === 'mouse') isPointerInside = true;
	}

	function notePictureUnderPointer(event: PointerEvent): void {
		const touched = event.target;
		const isMessagePicture = touched instanceof HTMLImageElement && touched.dataset.messagePicture !== undefined;
		pictureUnderPointer = isMessagePicture ? touched.currentSrc || touched.src : '';
	}

	async function openPickerOnceTheMenuHasClosed(): Promise<void> {
		await tick();
		isAnchoredPickerOpen = true;
	}
</script>

{#snippet toolbar()}
	<MessageToolbar
		canChange={canChangeThis}
		{canReply}
		{canDelete}
		copyKind={copyable.kind}
		bind:open={isToolbarMenuOpen}
		onQuickReact={onReact}
		{onReply}
		onCopy={() => onCopy(copyable)}
		{onDelete}
	>
		{#snippet addReaction()}
			<EmojiPicker bind:open={isToolbarPickerOpen} onPick={onReact} side="top" align={mine ? 'end' : 'start'}>
				{#snippet trigger({ props })}
					<Button {...props} variant="ghost" size="icon-xs" aria-label={text.addReaction}>
						<SmilePlusIcon />
					</Button>
				{/snippet}
			</EmojiPicker>
		{/snippet}
	</MessageToolbar>
{/snippet}

<div
	role="group"
	data-message-id={message.id}
	data-held={isHeld}
	class="group/row relative -mx-4 px-4 py-1 transition-colors data-[held=true]:z-10 data-[held=true]:bg-muted/40 [@media(hover:hover)]:hover:z-10 [@media(hover:hover)]:hover:bg-muted/40"
	onpointerenter={notePointerInside}
	onpointerleave={() => (isPointerInside = false)}
	onpointerdown={notePictureUnderPointer}
	use:swipeToReply={{ onReply, disabled: !canReply || !isSettled }}
>
	<MessageContextMenu
		canChange={canChangeThis}
		{canReply}
		{canDelete}
		canCopyText={copyable.kind === 'text'}
		canCopyPicture={pictureToCopy !== ''}
		disabled={!isSettled || !hasActions}
		bind:open={isContextMenuOpen}
		onQuickReact={onReact}
		onOpenPicker={openPickerOnceTheMenuHasClosed}
		{onReply}
		onCopyText={() => onCopy(copyable)}
		onCopyPicture={() => onCopy({ kind: 'picture', address: pictureToCopy })}
		{onDelete}
	>
		<div
			bind:this={contentElement}
			class="translate-x-(--swipe-offset,0px) group-[:not([data-swipe-armed])]/row:transition-transform"
			use:swallowClickAfterTouchHold={{ isMenuOpen: isContextMenuOpen }}
		>
			<Message.Root align={mine ? 'end' : 'start'}>
				{#if !mine}
					{#if endsGroup}
						{@render avatar()}
					{:else}
						<Message.Avatar />
					{/if}
				{/if}
				<Message.Content>
					{#if hasHeader}
						<Message.Header class="-mb-2 px-0">
							<span bind:clientWidth={measuredNameWidthPixels}>{senderName}</span>
						</Message.Header>
					{/if}
					{@render children({ nameWidthPixels: hasHeader ? measuredNameWidthPixels : 0, hasFooter, isToolbarShown, toolbar })}
					{#if hasFooter}
						<Message.Footer class="px-0">{@render footer()}</Message.Footer>
					{/if}
				</Message.Content>
			</Message.Root>
		</div>
	</MessageContextMenu>
	{#if canReply}
		<div
			aria-hidden="true"
			class="text-muted-foreground group-data-[swipe-armed=true]/row:bg-foreground group-data-[swipe-armed=true]/row:text-background pointer-events-none absolute top-1/2 right-4 flex size-9 -translate-y-1/2 items-center justify-center rounded-full opacity-(--swipe-progress,0) transition-colors"
		>
			<ReplyIcon class="size-4" />
		</div>
	{/if}
	{#if canChangeThis}
		<EmojiPicker
			bind:open={isAnchoredPickerOpen}
			onPick={onReact}
			customAnchor={contentElement}
			side="top"
			align={mine ? 'end' : 'start'}
		/>
	{/if}
</div>

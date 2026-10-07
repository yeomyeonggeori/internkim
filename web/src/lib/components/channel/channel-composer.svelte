<script lang="ts" module>
	import type { ChannelOutgoingAttachment } from './channel-api';
	import type { DraftMentions } from '$lib/messenger/mention-draft';

	export type OutgoingMessage = {
		text: string;
		attachments: ChannelOutgoingAttachment[];
		mentions: DraftMentions;
		attachmentSummary: string;
	};
</script>

<script lang="ts">
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import ComposerFormatButtons from './composer-format-buttons.svelte';
	import PaperclipIcon from '@lucide/svelte/icons/paperclip';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Toggle } from '$lib/components/ui/toggle/index.js';
	import MentionPopup from './mention-popup.svelte';
	import EmojiPicker from './emoji-picker.svelte';
	import ComposerFormatToolbar from './composer-format-toolbar.svelte';
	import ComposerEditor from './composer-editor.svelte';
	import type { ComposerFormat } from './composer-formatting';
	import { canChangeMessages } from './channel-api';
	import { fileToAttachment, formatAttachmentMeta } from './channel-attachments';
	import type { MentionCandidate, MentionPerson } from '$lib/messenger/mention-candidates';
	import { mentionKeyAction } from '$lib/messenger/mention-draft';
	import { createMentionPicker } from '$lib/messenger/mention-picker.svelte';
	import { isFormatToolbarShown, rememberFormatToolbarShown } from '$lib/messenger/format-toolbar-visibility';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import AtSignIcon from '@lucide/svelte/icons/at-sign';
	import CaseSensitiveIcon from '@lucide/svelte/icons/case-sensitive';
	import CropIcon from '@lucide/svelte/icons/crop';
	import FileIcon from '@lucide/svelte/icons/file';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SmilePlusIcon from '@lucide/svelte/icons/smile-plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { onDestroy, onMount } from 'svelte';

	let {
		name,
		placeholder,
		participants,
		isGroup,
		disabled = false,
		isSending = $bindable(false),
		onSend,
		onTyping,
		onCapture
	}: {
		name: string;
		placeholder: string;
		participants: MentionPerson[];
		isGroup: boolean;
		disabled?: boolean;
		isSending?: boolean;
		onSend: (outgoing: OutgoingMessage) => Promise<void>;
		onTyping?: () => void;
		onCapture?: () => void;
	} = $props();

	type PendingAttachment = {
		id: string;
		previewURL: string;
		isImage: boolean;
		sizeBytes: number;
		attachment: ChannelOutgoingAttachment;
	};

	const text = createPageText(channelText);
	const isMobile = new IsMobile(640);
	let showsSelectionFormatting = $state(false);
	let selectionAnchor = $state<HTMLElement | null>(null);
	let value = $state('');
	let composerEditor = $state<ComposerEditor | null>(null);
	let activeFormats = $state<ComposerFormat[]>([]);
	let fileInput = $state<HTMLInputElement | null>(null);
	let form = $state<HTMLFormElement | null>(null);
	let pendingAttachments = $state<PendingAttachment[]>([]);
	let attachmentSerial = 0;
	let showsFormatToolbar = $state(false);
	const formatToggleLabel = $derived(showsFormatToolbar ? text.hideFormatting : text.showFormatting);
	const canMention = $derived(canChangeMessages() && participants.length > 0);
	const mentions = createMentionPicker(() => participants, () => isGroup);
	const editorAriaAttributes = $derived<Record<string, string>>({
		role: 'combobox',
		'aria-label': placeholder,
		'aria-autocomplete': 'list',
		'aria-expanded': String(mentions.isOpen),
		'aria-controls': `mention-list-${name}`,
		...(mentions.isOpen ? { 'aria-activedescendant': `mention-row-${name}-${mentions.active}` } : {})
	});
	export function clear(): void {
		clearAttachments();
		value = '';
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const trimmed = value.trim();
		const attachments = pendingAttachments.map((pending) => pending.attachment);
		if ((!trimmed && attachments.length === 0) || isSending) return;
		const outgoingMentions = mentions.mentionsIn(trimmed);
		isSending = true;
		value = '';
		mentions.forget();
		const attachmentSummary = attachments.map((attachment) => attachment.filename).join(', ');
		clearAttachments();
		await onSend({ text: trimmed, attachments, mentions: outgoingMentions, attachmentSummary }).finally(
			() => (isSending = false)
		);
	}

	function filesFromInput(event: Event): File[] {
		if (!(event.currentTarget instanceof HTMLInputElement)) return [];
		const files = Array.from(event.currentTarget.files ?? []);
		event.currentTarget.value = '';
		return files;
	}

	async function handleFilesSelected(event: Event) {
		const built: PendingAttachment[] = [];
		for (const file of filesFromInput(event)) {
			built.push({
				id: `attachment-${attachmentSerial++}`,
				previewURL: URL.createObjectURL(file),
				isImage: file.type.startsWith('image/'),
				sizeBytes: file.size,
				attachment: await fileToAttachment(file)
			});
		}
		pendingAttachments = [...pendingAttachments, ...built];
	}

	function removeAttachment(id: string) {
		const removed = pendingAttachments.find((pending) => pending.id === id);
		if (removed) URL.revokeObjectURL(removed.previewURL);
		pendingAttachments = pendingAttachments.filter((pending) => pending.id !== id);
	}

	function clearAttachments() {
		for (const pending of pendingAttachments) URL.revokeObjectURL(pending.previewURL);
		pendingAttachments = [];
	}

	function handleInput(): void {
		refreshMentions();
		if (value.trim() !== '') onTyping?.();
	}

	function refreshMentions(): void {
		const before = composerEditor?.textBeforeCursor();
		if (!canMention || !before) return mentions.close();
		mentions.reopen(before.text, before.cursor);
	}

	function handleSelectionChange(): void {
		refreshMentions();
		selectionAnchor = composerEditor?.selectionElement() ?? null;
		showsSelectionFormatting = isMobile.current && !disabled && selectionAnchor !== null;
	}

	function takeMention(candidate?: MentionCandidate): void {
		const before = composerEditor?.textBeforeCursor();
		if (!before) return;
		const written = mentions.take(before.cursor, candidate);
		if (written) composerEditor?.writeMention(written);
	}

	function format(chosen: ComposerFormat): void {
		composerEditor?.format(chosen, () => window.prompt(text.formatLinkAddress));
	}

	function insertEmoji(glyph: string): void {
		composerEditor?.insertText(glyph);
	}

	function startMention(): void {
		const before = composerEditor?.textBeforeCursor();
		if (!before) return;
		const needsSpace = before.cursor > 0 && !/\s/.test(before.text[before.cursor - 1]);
		composerEditor?.insertText(needsSpace ? ' @' : '@');
		refreshMentions();
	}

	function handledByMentions(event: KeyboardEvent): boolean {
		if (!mentions.isOpen) return false;
		const action = mentionKeyAction(event.key, event.isComposing);
		if (!action) return false;
		event.preventDefault();
		if (action === 'close') mentions.close();
		else if (action === 'down') mentions.moveBy(1);
		else if (action === 'up') mentions.moveBy(-1);
		else takeMention();
		return true;
	}

	function handleKeydown(event: KeyboardEvent): boolean {
		if (handledByMentions(event)) return true;
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return false;
		event.preventDefault();
		form?.requestSubmit();
		return true;
	}

	onMount(() => {
		showsFormatToolbar = isFormatToolbarShown();
	});
	onDestroy(clearAttachments);
</script>

<form bind:this={form} onsubmit={submit} class="channel-composer relative px-3 py-2 sm:p-3">
	<input bind:this={fileInput} type="file" multiple class="hidden" onchange={handleFilesSelected} />
	{#if pendingAttachments.length > 0}
		<Attachment.Group class="mb-2">
			{#each pendingAttachments as pending (pending.id)}
				<Attachment.Root size="sm">
					{#if pending.isImage}
						<Attachment.Media variant="image">
							<img src={pending.previewURL} alt="" />
						</Attachment.Media>
					{:else}
						<Attachment.Media>
							<FileIcon />
						</Attachment.Media>
					{/if}
					<Attachment.Content>
						<Attachment.Title>{pending.attachment.filename}</Attachment.Title>
						<Attachment.Description>
							{formatAttachmentMeta({
								mimeType: pending.attachment.contentType,
								filename: pending.attachment.filename,
								sizeBytes: pending.sizeBytes
							})}
						</Attachment.Description>
					</Attachment.Content>
					<Attachment.Actions>
						<Attachment.Action aria-label={text.removeAttachment} onclick={() => removeAttachment(pending.id)}>
							<XIcon />
						</Attachment.Action>
					</Attachment.Actions>
				</Attachment.Root>
			{/each}
		</Attachment.Group>
	{/if}
	{#if mentions.isOpen}
		<MentionPopup
			{name}
			rows={mentions.rows}
			active={mentions.active}
			listLabel={text.mentionList}
			everyoneLabel={text.mentionEveryone}
			onPick={(candidate) => takeMention(candidate)}
		/>
	{/if}
	<InputGroup.Root class="composer-input frosted-surface">
		<ComposerEditor
			bind:this={composerEditor}
			bind:value
			bind:activeFormats
			placeholder={disabled ? text.composerDisabledPlaceholder : placeholder}
			ariaAttributes={editorAriaAttributes}
			{disabled}
			onKeydown={handleKeydown}
			onInput={handleInput}
			onSelectionChange={handleSelectionChange}
			onBlur={() => mentions.close()}
		/>
		<InputGroup.Addon align="inline-start" class="composer-tools-start sm:hidden">
			<InputGroup.Button
				variant="ghost"
				size="icon-sm"
				aria-label={text.addAttachment}
				onclick={() => fileInput?.click()}
				{disabled}
			>
				<PlusIcon />
			</InputGroup.Button>
		</InputGroup.Addon>
		<InputGroup.Addon align="inline-end" class="composer-send-end sm:hidden">
			{@render sendButton('')}
		</InputGroup.Addon>
		{#if showsFormatToolbar && !isMobile.current}
			<ComposerFormatToolbar {disabled} {activeFormats} onFormat={format} />
		{/if}
		<InputGroup.Addon align="block-end" class="composer-actions hidden pt-1 sm:flex">
			<div class="composer-tools" role="group" aria-label={text.composerTools}>
				{#if canMention}
					<InputGroup.Button variant="ghost" size="icon-sm" aria-label={text.addMention} {disabled} onclick={() => void startMention()}>
						<AtSignIcon />
					</InputGroup.Button>
				{/if}
				<InputGroup.Button
					type="button"
					variant="ghost"
					size="icon-sm"
					aria-label={text.addAttachment}
					onclick={() => fileInput?.click()}
					{disabled}
				>
					<PaperclipIcon />
				</InputGroup.Button>
				<EmojiPicker onPick={insertEmoji} side="top" align="start">
					{#snippet trigger({ props })}
						<InputGroup.Button {...props} variant="ghost" size="icon-sm" aria-label={text.addEmoji} {disabled}>
							<SmilePlusIcon />
						</InputGroup.Button>
					{/snippet}
				</EmojiPicker>
				{#if onCapture}
					<InputGroup.Button variant="ghost" size="icon-sm" aria-label={text.captureConversation} {disabled} onclick={onCapture}>
						<CropIcon />
					</InputGroup.Button>
				{/if}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<Toggle
								{...props}
								aria-label={formatToggleLabel}
								bind:pressed={showsFormatToolbar}
								onPressedChange={rememberFormatToolbarShown}
								{disabled}
							>
								<CaseSensitiveIcon />
							</Toggle>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content side="top">{formatToggleLabel}</Tooltip.Content>
				</Tooltip.Root>
			</div>
			{@render sendButton('ms-auto hidden sm:inline-flex')}
		</InputGroup.Addon>
	</InputGroup.Root>
	{#if isMobile.current}
		<span id={`composer-selection-help-${name}`} class="sr-only">{text.selectionFormattingHelp}</span>
		<Popover.Root bind:open={showsSelectionFormatting}>
			<Popover.Content
				customAnchor={selectionAnchor ?? undefined}
				side="top"
				align="center"
				trapFocus={false}
				role="dialog"
				aria-modal="false"
				aria-label={text.selectionFormatting}
				class="max-w-[calc(100vw-16px)] w-72"
				onOpenAutoFocus={(event) => event.preventDefault()}
				onCloseAutoFocus={(event) => event.preventDefault()}
				onEscapeKeydown={() => composerEditor?.focusSelection()}
			>
				<ComposerFormatButtons {disabled} {activeFormats} onFormat={format} onClear={() => composerEditor?.clearFormatting()} onPointerdown={(event) => event.preventDefault()} />
			</Popover.Content>
		</Popover.Root>
	{/if}
</form>

{#snippet sendButton(className: string)}
	<InputGroup.Button
		type="submit"
		variant="default"
		size="icon-sm"
		class={className}
		disabled={disabled || (value.trim().length === 0 && pendingAttachments.length === 0) || isSending}
	>
		<ArrowUpIcon />
		<span class="sr-only">{text.send}</span>
	</InputGroup.Button>
{/snippet}

<style>
	.channel-composer :global(.composer-tools) {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}
	@media (max-width: 639px) {
		.channel-composer :global(.composer-input) {
			display: grid;
			grid-template-columns: 32px minmax(0, 1fr) 32px;
			align-items: end;
			column-gap: 4px;
			min-height: 0;
			height: auto;
			padding: 3px;
			border-radius: 20px;
		}
		.channel-composer :global(.composer-tools-start),
		.channel-composer :global(.composer-send-end) {
			grid-row: 1;
			margin: 0;
			padding: 0;
		}
		.channel-composer :global(.composer-tools-start) {
			grid-column: 1;
		}
		.channel-composer :global(.composer-send-end) {
			grid-column: 3;
		}
		.channel-composer :global(.composer-input button) {
			width: 32px;
			min-width: 32px;
			height: 32px;
			min-height: 32px;
			border-radius: 9999px;
			transition-property: transform, opacity, background-color, color;
			transition-duration: 150ms;
			transition-timing-function: var(--ease-out-strong);
		}
		.channel-composer :global(.composer-input button:active:not(:disabled)) {
			transform: scale(0.92);
			transition-duration: 100ms;
		}
		.channel-composer :global([data-slot="input-group-control"]) {
			grid-column: 2;
			grid-row: 1;
		}
		.channel-composer :global(.tiptap) {
			min-height: 32px;
			max-height: min(160px, 30dvh);
			overflow-y: auto;
			padding: 6px 4px;
			font-size: 16px;
			line-height: 20px;
		}
		.channel-composer :global(.tiptap p) {
			margin-block: 0;
		}
		.channel-composer :global([data-slot="input-group-control"] > span) {
			top: 6px;
			left: 4px;
			line-height: 20px;
		}
	}
	@media (max-width: 639px) and (prefers-reduced-motion: reduce) {
		.channel-composer :global(.composer-input button:active:not(:disabled)) {
			transform: none;
		}
	}
</style>

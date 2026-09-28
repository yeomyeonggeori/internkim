<script lang="ts" module>
	import type { ChannelMessage, ChannelOutgoingAttachment } from './channel-api';
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
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import MentionPopup from './mention-popup.svelte';
	import { composerEditing, type EditingMessage } from './message-edit';
	import { canChangeMessages } from './channel-api';
	import { fileToAttachment, formatAttachmentMeta } from './channel-attachments';
	import type { MentionCandidate, MentionPerson } from '$lib/messenger/mention-candidates';
	import { mentionKeyAction } from '$lib/messenger/mention-draft';
	import { createMentionPicker } from '$lib/messenger/mention-picker.svelte';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import FileIcon from '@lucide/svelte/icons/file';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { onDestroy, tick } from 'svelte';

	let {
		name,
		placeholder,
		rows,
		participants,
		isGroup,
		disabled,
		cancelsEditOnEscape,
		isSending = $bindable(false),
		editing = $bindable(null),
		saveEdit,
		onSend
	}: {
		name: string;
		placeholder: string;
		rows: number;
		participants: MentionPerson[];
		isGroup: boolean;
		disabled: boolean;
		cancelsEditOnEscape: boolean;
		isSending?: boolean;
		editing?: EditingMessage | null;
		saveEdit: (messageID: string, text: string) => Promise<boolean>;
		onSend: (outgoing: OutgoingMessage) => Promise<void>;
	} = $props();

	type PendingAttachment = {
		id: string;
		previewURL: string;
		isImage: boolean;
		sizeBytes: number;
		attachment: ChannelOutgoingAttachment;
	};

	const text = createPageText(channelText);
	let value = $state('');
	let textarea = $state<HTMLTextAreaElement | null>(null);
	let fileInput = $state<HTMLInputElement | null>(null);
	let pendingAttachments = $state<PendingAttachment[]>([]);
	let attachmentSerial = 0;
	const canMention = $derived(canChangeMessages() && participants.length > 0);
	const mentions = createMentionPicker(() => participants, () => isGroup);
	const edit = composerEditing({
		text: () => value,
		setText: (written) => (value = written),
		editing: () => editing,
		setEditing: (next) => (editing = next),
		focus: () => void tick().then(() => textarea?.focus())
	});

	export function beginEdit(message: ChannelMessage): void {
		edit.begin(message);
	}

	export function cancelEdit(): void {
		edit.cancel();
	}

	export function clear(): void {
		clearAttachments();
		value = '';
		editing = null;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (editing) {
			if (isSending) return;
			isSending = true;
			await edit.save(saveEdit).finally(() => (isSending = false));
			return;
		}
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

	function refreshMentions(): void {
		if (!canMention || !textarea) return mentions.close();
		mentions.reopen(textarea.value, textarea.selectionStart ?? textarea.value.length);
	}

	async function takeMention(candidate?: MentionCandidate): Promise<void> {
		const element = textarea;
		if (!element) return;
		const written = mentions.take(element.value, element.selectionStart ?? element.value.length, candidate);
		if (!written) return;
		value = written.text;
		await tick();
		element.focus();
		element.setSelectionRange(written.cursor, written.cursor);
	}

	function handledByMentions(event: KeyboardEvent): boolean {
		if (!mentions.isOpen) return false;
		const action = mentionKeyAction(event.key, event.isComposing);
		if (!action) return false;
		event.preventDefault();
		if (action === 'close') mentions.close();
		else if (action === 'down') mentions.moveBy(1);
		else if (action === 'up') mentions.moveBy(-1);
		else void takeMention();
		return true;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (handledByMentions(event)) return;
		if (event.key === 'Escape' && editing && cancelsEditOnEscape) {
			event.preventDefault();
			edit.cancel();
			return;
		}
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
		event.preventDefault();
		if (!(event.currentTarget instanceof HTMLElement)) return;
		event.currentTarget.closest('form')?.requestSubmit();
	}

	onDestroy(clearAttachments);
</script>

<form onsubmit={submit} class="relative border-t p-3">
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
			onPick={(candidate) => void takeMention(candidate)}
		/>
	{/if}
	{#if editing}
		<div class="text-muted-foreground flex items-center justify-between gap-2 text-xs">
			<span>{text.editingMessage}</span>
			<Button type="button" variant="ghost" size="xs" onclick={edit.cancel}>{text.cancelEdit}</Button>
		</div>
	{/if}
	<InputGroup.Root>
		<InputGroup.Textarea
			bind:value
			bind:ref={textarea}
			role="combobox"
			aria-autocomplete="list"
			aria-expanded={mentions.isOpen}
			aria-controls={`mention-list-${name}`}
			aria-activedescendant={mentions.isOpen ? `mention-row-${name}-${mentions.active}` : undefined}
			placeholder={disabled ? text.composerDisabledPlaceholder : placeholder}
			aria-label={placeholder}
			{rows}
			onkeydown={handleKeydown}
			oninput={refreshMentions}
			onblur={() => mentions.close()}
			{disabled}
		/>
		<InputGroup.Addon align="block-end" class="pt-1">
			<InputGroup.Button
				type="button"
				variant="outline"
				size="icon-sm"
				aria-label={text.addAttachment}
				onclick={() => fileInput?.click()}
				disabled={disabled || editing !== null}
			>
				<PlusIcon />
			</InputGroup.Button>
			<InputGroup.Button
				type="submit"
				variant="default"
				size="icon-sm"
				class="ms-auto"
				disabled={disabled || (value.trim().length === 0 && pendingAttachments.length === 0) || isSending}
			>
				<ArrowUpIcon />
				<span class="sr-only">{editing ? text.saveEdit : text.send}</span>
			</InputGroup.Button>
		</InputGroup.Addon>
	</InputGroup.Root>
</form>

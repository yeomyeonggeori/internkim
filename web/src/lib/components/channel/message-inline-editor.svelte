<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { MentionCandidate, MentionPerson } from '$lib/messenger/mention-candidates';
	import { mentionKeyAction, type DraftMentions } from '$lib/messenger/mention-draft';
	import { createMentionPicker } from '$lib/messenger/mention-picker.svelte';
	import MentionPopup from './mention-popup.svelte';
	import { onMount, tick, untrack } from 'svelte';

	let {
		originalText,
		originalMentions,
		participants = [],
		isGroup = false,
		onSave,
		onCancel
	}: {
		originalText: string;
		originalMentions?: DraftMentions;
		participants?: MentionPerson[];
		isGroup?: boolean;
		onSave: (editedText: string, mentions: DraftMentions) => Promise<void>;
		onCancel: () => void;
	} = $props();

	const popupName = 'inline-edit';
	const text = createPageText(channelText);
	const mentions = createMentionPicker(() => participants, () => isGroup);
	let draft = $state(untrack(() => originalText));
	let isSaving = $state(false);
	let field = $state<HTMLTextAreaElement | null>(null);
	const canMention = $derived(participants.length > 0);

	onMount(() => {
		mentions.remember(originalMentions);
		field?.focus();
		field?.setSelectionRange(draft.length, draft.length);
	});

	async function save(): Promise<void> {
		if (isSaving) return;
		isSaving = true;
		await onSave(draft, mentions.mentionsIn(draft)).finally(() => (isSaving = false));
	}

	function cursor(): number {
		return field?.selectionStart ?? draft.length;
	}

	function refreshMentions(): void {
		if (!canMention) return mentions.close();
		mentions.reopen(draft, cursor());
	}

	async function takeMention(candidate?: MentionCandidate): Promise<void> {
		const written = mentions.take(cursor(), candidate);
		if (!written) return;
		draft = draft.slice(0, written.from) + written.inserted + draft.slice(written.to);
		const caret = written.from + written.inserted.length;
		await tick();
		field?.focus();
		field?.setSelectionRange(caret, caret);
	}

	function handledByMentions(event: KeyboardEvent): boolean {
		if (!mentions.isOpen) return false;
		const action = mentionKeyAction(event.key, event.isComposing);
		if (!action) return false;
		event.preventDefault();
		event.stopPropagation();
		if (action === 'close') mentions.close();
		else if (action === 'down') mentions.moveBy(1);
		else if (action === 'up') mentions.moveBy(-1);
		else void takeMention();
		return true;
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (handledByMentions(event)) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			onCancel();
			return;
		}
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
		event.preventDefault();
		void save();
	}
</script>

<div class="relative flex w-full max-w-[min(100%,36rem)] flex-col gap-2 self-start group-data-[align=end]/message:self-end">
	{#if mentions.isOpen}
		<MentionPopup
			name={popupName}
			rows={mentions.rows}
			active={mentions.active}
			listLabel={text.mentionList}
			everyoneLabel={text.mentionEveryone}
			onPick={(candidate) => void takeMention(candidate)}
		/>
	{/if}
	<Textarea
		bind:ref={field}
		bind:value={draft}
		aria-label={text.editingMessage}
		disabled={isSaving}
		onkeydown={handleKeydown}
		oninput={refreshMentions}
		onclick={refreshMentions}
		onblur={() => mentions.close()}
		class="max-h-60 resize-none [field-sizing:content]"
	/>
	<div class="flex items-center justify-end gap-2">
		<Button type="button" variant="ghost" onclick={onCancel}>{text.cancel}</Button>
		<Button type="button" disabled={isSaving} onclick={() => void save()}>{text.saveEdit}</Button>
	</div>
</div>

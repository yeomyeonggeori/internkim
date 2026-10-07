<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount, untrack } from 'svelte';

	let {
		originalText,
		onSave,
		onCancel
	}: {
		originalText: string;
		onSave: (editedText: string) => Promise<void>;
		onCancel: () => void;
	} = $props();

	const text = createPageText(channelText);
	let draft = $state(untrack(() => originalText));
	let isSaving = $state(false);
	let field = $state<HTMLTextAreaElement | null>(null);

	onMount(() => {
		field?.focus();
		field?.setSelectionRange(draft.length, draft.length);
	});

	async function save(): Promise<void> {
		if (isSaving) return;
		isSaving = true;
		await onSave(draft).finally(() => (isSaving = false));
	}

	function handleKeydown(event: KeyboardEvent): void {
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

<div class="flex w-full max-w-[min(100%,36rem)] flex-col gap-2 self-start group-data-[align=end]/message:self-end">
	<Textarea
		bind:ref={field}
		bind:value={draft}
		aria-label={text.editingMessage}
		disabled={isSaving}
		onkeydown={handleKeydown}
		class="max-h-60 resize-none [field-sizing:content]"
	/>
	<div class="flex items-center justify-end gap-2">
		<Button type="button" variant="ghost" onclick={onCancel}>{text.cancel}</Button>
		<Button type="button" disabled={isSaving} onclick={() => void save()}>{text.saveEdit}</Button>
	</div>
</div>

<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as Field from '$lib/components/ui/field';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { deleteMemoryFact, updateMemoryFact, type MemoryGraphEpisode, type MemoryGraphFact } from './memory-graph-api';
	import { isCurrentMemoryFact, memoryAudience, memoryDate, memorySources } from './memory-workbench-model';
	import type { MemoryText } from './text';

	let { fact, episodes, text, onChanged }: { fact: MemoryGraphFact; episodes: MemoryGraphEpisode[]; text: MemoryText; onChanged: () => Promise<void> } = $props();
	let isEditing = $state(false);
	let isSaving = $state(false);
	let draftContent = $state('');
	let errorMessage = $state('');
	const sources = $derived(memorySources(fact, episodes));

	function beginEdit(): void {
		draftContent = fact.content;
		errorMessage = '';
		isEditing = true;
	}

	async function saveMemory(): Promise<void> {
		if (isSaving || !draftContent.trim()) return;
		isSaving = true;
		errorMessage = '';
		try {
			await updateMemoryFact(fact.factID, fact.namespaceID, draftContent.trim());
			isEditing = false;
			await onChanged();
		} catch { errorMessage = text.memorySaveFailed; }
		finally { isSaving = false; }
	}

	function confirmRemoval(): void {
		confirmDelete({ title: text.memoryDeleteTitle, description: text.factOnlyDeleteDescription,
			confirm: { text: text.memoryDeleteAction }, cancel: { text: text.cancel }, onConfirm: removeMemory });
	}

	async function removeMemory(): Promise<void> {
		isSaving = true;
		errorMessage = '';
		try {
			await deleteMemoryFact(fact.factID, fact.namespaceID);
			await onChanged();
		} catch { errorMessage = text.memoryDeleteFailed; }
		finally { isSaving = false; }
	}
</script>

<div class="flex flex-col gap-6 px-5 py-5 sm:px-6">
	<div class="flex flex-wrap items-center gap-2"><Badge variant="secondary">{memoryAudience(fact, text)}</Badge><span class="text-xs text-muted-foreground">{isCurrentMemoryFact(fact) ? text.current : text.previousMemory}</span></div>
	{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
	{#if isEditing}
		<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); void saveMemory(); }}>
			<Field.Field>
				<Field.FieldLabel for="memory-content">{text.memoryEdit}</Field.FieldLabel>
				<Textarea id="memory-content" class="min-h-40" maxlength={600} bind:value={draftContent} disabled={isSaving} />
			</Field.Field>
			<div class="flex gap-2"><Button type="submit" disabled={isSaving || !draftContent.trim()}>{text.save}</Button><Button variant="outline" disabled={isSaving} onclick={() => isEditing = false}>{text.cancel}</Button></div>
		</form>
	{:else}
		<div class="memory-markdown break-words text-base leading-7"><SvelteMarkdown source={fact.content} /></div>
		<div class="flex gap-2"><Button variant="outline" size="sm" disabled={isSaving} onclick={beginEdit}><PencilIcon data-icon="inline-start" />{text.memoryEdit}</Button><Button variant="ghost" size="sm" disabled={isSaving} onclick={confirmRemoval}><TrashIcon data-icon="inline-start" />{text.memoryDelete}</Button></div>
	{/if}
	<dl class="grid grid-cols-2 gap-4 border-t pt-5 text-sm">
		<div class="grid gap-1"><dt class="text-xs text-muted-foreground">{text.recordedAt}</dt><dd>{memoryDate(fact.recordedAt, text, currentLocale.value)}</dd></div>
		<div class="grid gap-1"><dt class="text-xs text-muted-foreground">{fact.invalidAt || fact.expiredAt ? text.validity : text.validFrom}</dt><dd>{memoryDate(fact.validAt, text, currentLocale.value)}{#if fact.invalidAt || fact.expiredAt} → {memoryDate(fact.invalidAt ?? fact.expiredAt, text, currentLocale.value)}{/if}</dd></div>
	</dl>
	<section class="grid gap-3 border-t pt-5" aria-label={text.source}>
		<h2 class="text-sm font-medium">{text.source}</h2>
		{#each sources as source (source.episodeID)}
			<div class="grid gap-2">
				<p class="text-xs text-muted-foreground">{memoryDate(source.occurredAt, text, currentLocale.value)}{#if source.platform} · {source.platform}{/if}</p>
				<p class="whitespace-pre-wrap break-words text-sm leading-6">{source.prompt || text.sourceUnavailable}</p>
			</div>
		{:else}<p class="text-sm leading-6 text-muted-foreground">{text.sourceUnavailable}</p>{/each}
	</section>
</div>

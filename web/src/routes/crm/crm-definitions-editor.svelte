<script lang="ts">
	import DefinitionListCard from '$lib/components/definition-list-card.svelte';
	import {
		appendCRMDefinition,
		removeCRMDefinition,
		updateCRMDefinition,
		type CRMDefinitionCollection,
		type CRMDefinitionTarget,
		type CRMDefinitionsText,
		type CRMVocabulary
	} from './crm-definitions';
	import { CRMDefinitionsDraft } from './crm-definitions-draft.svelte';

	type Props = {
		vocabulary: CRMVocabulary;
		isSaving?: boolean;
		errorMessage?: string;
		text: CRMDefinitionsText;
		onSave: (vocabulary: CRMVocabulary) => Promise<void> | void;
	};

	let { vocabulary, isSaving = false, errorMessage = '', text, onSave }: Props = $props();

	const draft = new CRMDefinitionsDraft(
		() => vocabulary,
		(next) => onSave(next),
		() => text.nameRequired
	);

	let isBusy = $derived(isSaving || draft.isSaving);
	let saveState = $derived(isBusy ? text.saving : '');

	$effect(() => draft.synchronize());

	function rename(target: CRMDefinitionTarget, name: string): void {
		draft.commit(updateCRMDefinition(draft.value, target, { name }));
	}

	function changeColor(target: CRMDefinitionTarget, color: string): void {
		draft.commit(updateCRMDefinition(draft.value, target, { color }));
	}

	function remove(target: CRMDefinitionTarget): void {
		draft.commit(removeCRMDefinition(draft.value, target));
	}

	function add(collection: CRMDefinitionCollection, name: string, color: string): void {
		draft.commit(appendCRMDefinition(draft.value, collection, { id: crypto.randomUUID(), name, color }));
	}
</script>

<section class="space-y-4" aria-label={text.title}>
	<p class="sr-only" role="status" aria-live="polite">{saveState}</p>

	{#if errorMessage || draft.errorMessage}
		<p class="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive" role="alert">
			{draft.errorMessage || errorMessage}
		</p>
	{/if}

	<div class="grid gap-4 lg:grid-cols-2">
		<DefinitionListCard
			title={text.organizationTypes}
			items={draft.value.organization_types}
			{saveState}
			addLabel={text.add}
			removeLabel={text.remove}
			removeTitle={text.removeTitle}
			removeDescription={text.removeDescription}
			cancelLabel={text.cancel}
			colorLabel={text.color}
			doneLabel={text.done}
			emptyLabel={text.emptyList}
			onRename={(id, name) => rename({ kind: 'organization_type', id }, name)}
			onColorChange={(id, color) => changeColor({ kind: 'organization_type', id }, color)}
			onRemove={(id) => remove({ kind: 'organization_type', id })}
			onAdd={(name, color) => add('organization_type', name, color)}
		/>

		<DefinitionListCard
			title={text.pipelines}
			items={draft.value.pipelines}
			{saveState}
			addLabel={text.add}
			removeLabel={text.remove}
			removeTitle={text.removeTitle}
			removeDescription={text.removeDescription}
			cancelLabel={text.cancel}
			colorLabel={text.color}
			doneLabel={text.done}
			emptyLabel={text.emptyList}
			onRename={(id, name) => rename({ kind: 'pipeline', id }, name)}
			onColorChange={(id, color) => changeColor({ kind: 'pipeline', id }, color)}
			onRemove={(id) => remove({ kind: 'pipeline', id })}
			onAdd={(name, color) => add('pipeline', name, color)}
		/>
	</div>
</section>

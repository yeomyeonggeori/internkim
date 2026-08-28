<script lang="ts">
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
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
	import CRMDefinitionListCard from './crm-definition-list-card.svelte';

	type Props = {
		vocabulary: CRMVocabulary;
		isAdmin: boolean;
		isSaving?: boolean;
		errorMessage?: string;
		text: CRMDefinitionsText;
		onSave: (vocabulary: CRMVocabulary) => Promise<void> | void;
	};

	let { vocabulary, isAdmin, isSaving = false, errorMessage = '', text, onSave }: Props = $props();

	const draft = new CRMDefinitionsDraft(
		() => vocabulary,
		(next) => onSave(next),
		() => text.nameRequired
	);

	let isBusy = $derived(isSaving || draft.isSaving);

	$effect(() => draft.synchronize());

	function createID(): string {
		return crypto.randomUUID();
	}

	function commit(next: CRMVocabulary): void {
		if (!isAdmin) return;
		draft.commit(next);
	}

	function nameInput(target: CRMDefinitionTarget, name: string): void {
		draft.edit(updateCRMDefinition(draft.value, target, { name }));
	}

	function colorChange(target: CRMDefinitionTarget, color: string): void {
		commit(updateCRMDefinition(draft.value, target, { color }));
	}

	function remove(target: CRMDefinitionTarget): void {
		commit(removeCRMDefinition(draft.value, target));
	}

	function add(collection: CRMDefinitionCollection, name: string, color: string): void {
		commit(appendCRMDefinition(draft.value, collection, { id: createID(), name, color }));
	}
</script>

<section class="space-y-4" aria-labelledby="crm-definitions-title">
	<header class="space-y-1">
		<h2 id="crm-definitions-title" class="flex items-center gap-2 text-lg font-semibold">
			{text.title}
			{#if isBusy}<LoaderCircleIcon class="size-4 animate-spin text-muted-foreground" />{/if}
		</h2>
		<p class="text-sm text-muted-foreground">{text.description}</p>
	</header>

	<p class="sr-only" role="status" aria-live="polite">{isBusy ? text.saving : ''}</p>

	{#if !isAdmin}
		<p class="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">{text.readOnly}</p>
	{/if}
	{#if errorMessage || draft.errorMessage}
		<p class="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive" role="alert">
			{draft.errorMessage || errorMessage}
		</p>
	{/if}

	<div class="grid gap-4 lg:grid-cols-2">
		<CRMDefinitionListCard
			title={text.organizationTypes}
			description={text.organizationTypesDescription}
			items={draft.value.organization_types}
			{isAdmin}
			addLabel={text.add}
			removeLabel={text.remove}
			colorLabel={text.color}
			onNameInput={(id, name) => nameInput({ kind: 'organization_type', id }, name)}
			onCommit={() => commit(draft.value)}
			onColorChange={(id, color) => colorChange({ kind: 'organization_type', id }, color)}
			onRemove={(id) => remove({ kind: 'organization_type', id })}
			onAdd={(name, color) => add('organization_type', name, color)}
		/>

		<CRMDefinitionListCard
			title={text.pipelines}
			description={text.pipelinesDescription}
			items={draft.value.pipelines}
			{isAdmin}
			addLabel={text.add}
			removeLabel={text.remove}
			colorLabel={text.color}
			onNameInput={(id, name) => nameInput({ kind: 'pipeline', id }, name)}
			onCommit={() => commit(draft.value)}
			onColorChange={(id, color) => colorChange({ kind: 'pipeline', id }, color)}
			onRemove={(id) => remove({ kind: 'pipeline', id })}
			onAdd={(name, color) => add('pipeline', name, color)}
		/>
	</div>
</section>

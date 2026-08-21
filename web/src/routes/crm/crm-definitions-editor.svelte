<script lang="ts">
	import {
		appendCRMDefinition,
		cloneCRMVocabulary,
		removeCRMDefinition,
		updateCRMDefinition,
		type CRMDefinitionCollection,
		type CRMDefinitionDeleteRequest,
		type CRMDefinitionTarget,
		type CRMDefinitionsText,
		type CRMVocabulary
	} from './crm-definitions';
	import CRMDefinitionListCard from './crm-definition-list-card.svelte';

	type Props = {
		vocabulary: CRMVocabulary;
		isAdmin: boolean;
		isSaving?: boolean;
		errorMessage?: string;
		text: CRMDefinitionsText;
		onSave: (vocabulary: CRMVocabulary) => Promise<void> | void;
		onDelete: (
			request: CRMDefinitionDeleteRequest,
			vocabulary: CRMVocabulary
		) => Promise<void> | void;
	};

	let {
		vocabulary,
		isAdmin,
		isSaving = false,
		errorMessage = '',
		text,
		onSave,
		onDelete
	}: Props = $props();

	let draft = $state<CRMVocabulary>({ organization_types: [], pipelines: [] });
	let localErrorMessage = $state('');
	let isSubmitting = $state(false);
	let disabled = $derived(isSaving || isSubmitting);

	$effect(() => {
		draft = cloneCRMVocabulary(vocabulary);
	});

	function createID(): string {
		return crypto.randomUUID();
	}

	function hasBlankName(value: CRMVocabulary): boolean {
		return [
			...value.organization_types,
			...value.pipelines
		].some((definition) => !definition.name.trim());
	}

	async function save(next: CRMVocabulary): Promise<void> {
		if (!isAdmin || disabled) return;
		if (hasBlankName(next)) {
			localErrorMessage = text.nameRequired;
			return;
		}
		isSubmitting = true;
		localErrorMessage = '';
		try {
			await onSave(cloneCRMVocabulary(next));
			draft = next;
		} catch (error) {
			localErrorMessage = error instanceof Error ? error.message : String(error);
			draft = cloneCRMVocabulary(vocabulary);
		} finally {
			isSubmitting = false;
		}
	}

	function nameInput(target: CRMDefinitionTarget, name: string): void {
		draft = updateCRMDefinition(draft, target, { name });
	}

	function colorChange(target: CRMDefinitionTarget, color: string): void {
		void save(updateCRMDefinition(draft, target, { color }));
	}

	async function remove(target: CRMDefinitionTarget): Promise<void> {
		if (!isAdmin || disabled) return;
		const next = removeCRMDefinition(draft, target);
		isSubmitting = true;
		localErrorMessage = '';
		try {
			await onDelete(target, cloneCRMVocabulary(next));
			draft = next;
		} catch (error) {
			localErrorMessage = error instanceof Error ? error.message : String(error);
			draft = cloneCRMVocabulary(vocabulary);
		} finally {
			isSubmitting = false;
		}
	}

	function add(collection: CRMDefinitionCollection, name: string, color: string): void {
		void save(appendCRMDefinition(draft, collection, { id: createID(), name, color }));
	}
</script>

<section class="space-y-4" aria-labelledby="crm-definitions-title">
	<header class="space-y-1">
		<h2 id="crm-definitions-title" class="text-lg font-semibold">{text.title}</h2>
		<p class="text-sm text-muted-foreground">{text.description}</p>
	</header>

	{#if !isAdmin}
		<p class="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">{text.readOnly}</p>
	{/if}
	{#if errorMessage || localErrorMessage}
		<p class="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive" role="alert">
			{localErrorMessage || errorMessage}
		</p>
	{/if}
	{#if disabled}
		<p class="text-sm text-muted-foreground" aria-live="polite">{text.saving}</p>
	{/if}

	<div class="grid gap-4 lg:grid-cols-2">
		<CRMDefinitionListCard
			title={text.organizationTypes}
			description={text.organizationTypesDescription}
			items={draft.organization_types}
			{isAdmin}
			{disabled}
			addLabel={text.add}
			removeLabel={text.remove}
			colorLabel={text.color}
			onNameInput={(id, name) => nameInput({ kind: 'organization_type', id }, name)}
			onCommit={() => void save(draft)}
			onColorChange={(id, color) => colorChange({ kind: 'organization_type', id }, color)}
			onRemove={(id) => void remove({ kind: 'organization_type', id })}
			onAdd={(name, color) => add('organization_type', name, color)}
		/>

		<CRMDefinitionListCard
			title={text.pipelines}
			description={text.pipelinesDescription}
			items={draft.pipelines}
			{isAdmin}
			{disabled}
			addLabel={text.add}
			removeLabel={text.remove}
			colorLabel={text.color}
			onNameInput={(id, name) => nameInput({ kind: 'pipeline', id }, name)}
			onCommit={() => void save(draft)}
			onColorChange={(id, color) => colorChange({ kind: 'pipeline', id }, color)}
			onRemove={(id) => void remove({ kind: 'pipeline', id })}
			onAdd={(name, color) => add('pipeline', name, color)}
		/>
	</div>
</section>

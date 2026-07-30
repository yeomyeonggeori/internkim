<script lang="ts">
	import FlowEditableDefinitionListCard from './flow-editable-definition-list-card.svelte';
	import { flowBusinessColor, flowTaskTypeColor } from './flow-definition-colors';
	import FlowSizeDefinitionsCard from './flow-size-definitions-card.svelte';
	import type { FlowDefinitions } from './flow-types';

	type DefinitionsText = {
		size: string;
		sizeDescription: string;
		sizeName: string;
		distance: string;
		maxHours: string;
		developmentExample: string;
		otherExample: string;
		note: string;
		business: string;
		businessDescription: string;
		color: string;
		type: string;
		typeDescription: string;
		adminOnly: string;
		saving: string;
		save: string;
		removeAction: string;
		add: string;
		autoSave: string;
		saved: string;
	};
	type DefinitionSaveState = 'idle' | 'saving' | 'saved' | 'error';

	type Props = {
		definitions: FlowDefinitions;
		categoryDrafts: string[];
		setCategoryColor: (index: number, color: string) => void;
		setTypeColor: (index: number, color: string) => void;
		typeDrafts: string[];
		newCategoryText: string;
		newTypeText: string;
		isAdmin: boolean;
		canEditDefinitions: boolean;
		isSavingDefinitions: boolean;
		definitionSaveState: DefinitionSaveState;
		definitionErrorMessage: string;
		loadError: string;
		text: DefinitionsText;
		updateCategory: (index: number, value: string) => void;
		updateType: (index: number, value: string) => void;
		removeCategory: (index: number) => void;
		removeType: (index: number) => void;
		addCategory: () => void;
		addType: () => void;
		setNewCategoryText: (value: string) => void;
		setNewTypeText: (value: string) => void;
		saveDefinitions: () => void;
	};

	let {
		definitions,
		categoryDrafts,
		setCategoryColor,
		setTypeColor,
		typeDrafts,
		newCategoryText,
		newTypeText,
		isAdmin,
		canEditDefinitions,
		isSavingDefinitions,
		definitionSaveState,
		definitionErrorMessage,
		loadError,
		text,
		updateCategory,
		updateType,
		removeCategory,
		removeType,
		addCategory,
		addType,
		setNewCategoryText,
		setNewTypeText,
		saveDefinitions
	}: Props = $props();

	let definitionStatusMessage = $derived(definitionErrorMessage || definitionSaveStateMessage(definitionSaveState));
	let definitionStatusClass = $derived(definitionStatusContainerClass(definitionSaveState, Boolean(definitionErrorMessage)));

	function definitionSaveStateMessage(saveState: DefinitionSaveState): string {
		if (saveState === 'saving' || isSavingDefinitions) return text.saving;
		if (saveState === 'saved') return text.saved;
		return text.autoSave;
	}

	function definitionStatusContainerClass(saveState: DefinitionSaveState, hasError: boolean): string {
		if (hasError || saveState === 'error') return 'rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive';
		if (saveState === 'saved') return 'rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700';
		return 'rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground';
	}
</script>

{#if canEditDefinitions}
	<section class="grid gap-4">
		<FlowSizeDefinitionsCard {definitions} {text} />
		<div class="grid gap-4 lg:grid-cols-2">
			<FlowEditableDefinitionListCard
				title={text.business}
				description={text.businessDescription}
				items={categoryDrafts}
				newValue={newCategoryText}
				{isAdmin}
				removeLabel={text.removeAction}
				addLabel={text.add}
				update={updateCategory}
				remove={removeCategory}
				add={addCategory}
				setNewValue={setNewCategoryText}
				{saveDefinitions}
				itemColor={(index) => flowBusinessColor(categoryDrafts[index] ?? '', definitions)}
				setItemColor={setCategoryColor}
				colorLabel={text.color}
			/>
			<FlowEditableDefinitionListCard
				title={text.type}
				description={text.typeDescription}
				items={typeDrafts}
				newValue={newTypeText}
				{isAdmin}
				removeLabel={text.removeAction}
				addLabel={text.add}
				update={updateType}
				remove={removeType}
				add={addType}
				setNewValue={setNewTypeText}
				{saveDefinitions}
				itemColor={(index) => flowTaskTypeColor(typeDrafts[index] ?? '', definitions)}
				setItemColor={setTypeColor}
				colorLabel={text.color}
			/>
		</div>
		{#if isAdmin}
			<p class={definitionStatusClass}>
				{definitionStatusMessage}
			</p>
		{:else}
			<p class="text-sm text-muted-foreground">{text.adminOnly}</p>
		{/if}
	</section>
{:else}
	<div class="rounded-lg border bg-muted/30 p-4 text-sm text-muted-foreground">
		{loadError}
	</div>
{/if}

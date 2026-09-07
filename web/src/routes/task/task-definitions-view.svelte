<script lang="ts">
	import DefinitionListCard from '$lib/components/definition-list-card.svelte';
	import { taskBusinessColor, taskTypeColor } from './task-definition-colors';
	import TaskSizeDefinitionsCard from './task-size-definitions-card.svelte';
	import type { TaskDefinitions } from './task-types';

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
		etcLabel: string;
		color: string;
		type: string;
		adminOnly: string;
		saving: string;
		save: string;
		removeAction: string;
		removeBusinessTitle: string;
		removeBusinessDescription: string;
		removeTypeTitle: string;
		removeTypeDescription: string;
		cancel: string;
		done: string;
		add: string;
		autoSave: string;
		saved: string;
		saveError: string;
		emptyList: string;
	};
	type DefinitionSaveState = 'idle' | 'saving' | 'saved' | 'error';

	type Props = {
		definitions: TaskDefinitions;
		categoryDrafts: string[];
		setCategoryColor: (index: number, color: string) => void;
		setNewCategoryColor: (color: string) => void;
		setNewTypeColor: (color: string) => void;
		setTypeColor: (index: number, color: string) => void;
		typeDrafts: string[];
		etcBusinessColor: string;
		etcTypeColor: string;
		setEtcBusinessColor: (color: string) => void;
		setEtcTypeColor: (color: string) => void;
		isAdmin: boolean;
		canEditDefinitions: boolean;
		definitionSaveState: DefinitionSaveState;
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
		setNewCategoryColor,
		setNewTypeColor,
		setTypeColor,
		typeDrafts,
		etcBusinessColor,
		etcTypeColor,
		setEtcBusinessColor,
		setEtcTypeColor,
		isAdmin,
		canEditDefinitions,
		definitionSaveState,
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

	const businessItems = $derived(definitionItemsOf(categoryDrafts));
	const typeItems = $derived(definitionItemsOf(typeDrafts));
	const saveState = $derived(saveStateLabel());

	function definitionItemsOf(names: string[]): Array<{ id: string; name: string }> {
		return names.map((name, index) => ({ id: String(index), name }));
	}

	function saveStateLabel(): string {
		if (definitionSaveState === 'saving') return text.saving;
		if (definitionSaveState === 'saved') return text.saved;
		if (definitionSaveState === 'error') return text.saveError;
		return '';
	}

	function renameDefinition(update: (index: number, value: string) => void, id: string, name: string): void {
		update(Number(id), name);
		saveDefinitions();
	}
</script>

{#if canEditDefinitions}
	<section class="grid gap-4">
		<TaskSizeDefinitionsCard {text} />
		<div class="grid gap-4 lg:grid-cols-2">
			<DefinitionListCard
				title={text.business}
				items={businessItems}
				itemColor={(item) => taskBusinessColor(item.name, definitions)}
				isEditable={isAdmin}
				{saveState}
				addLabel={text.add}
				removeLabel={text.removeAction}
				removeTitle={text.removeBusinessTitle}
				removeDescription={text.removeBusinessDescription}
				cancelLabel={text.cancel}
				colorLabel={text.color}
				doneLabel={text.done}
				emptyLabel={text.emptyList}
				fallbackItem={{ name: text.etcLabel, color: etcBusinessColor }}
				onFallbackColorChange={setEtcBusinessColor}
				onRename={(id, name) => renameDefinition(updateCategory, id, name)}
				onColorChange={(id, color) => setCategoryColor(Number(id), color)}
				onRemove={(id) => removeCategory(Number(id))}
				onAdd={(name, color) => {
					setNewCategoryText(name);
					setNewCategoryColor(color);
					addCategory();
				}}
			/>
			<DefinitionListCard
				title={text.type}
				items={typeItems}
				itemColor={(item) => taskTypeColor(item.name, definitions)}
				isEditable={isAdmin}
				{saveState}
				addLabel={text.add}
				removeLabel={text.removeAction}
				removeTitle={text.removeTypeTitle}
				removeDescription={text.removeTypeDescription}
				cancelLabel={text.cancel}
				colorLabel={text.color}
				doneLabel={text.done}
				emptyLabel={text.emptyList}
				fallbackItem={{ name: text.etcLabel, color: etcTypeColor }}
				onFallbackColorChange={setEtcTypeColor}
				onRename={(id, name) => renameDefinition(updateType, id, name)}
				onColorChange={(id, color) => setTypeColor(Number(id), color)}
				onRemove={(id) => removeType(Number(id))}
				onAdd={(name, color) => {
					setNewTypeText(name);
					setNewTypeColor(color);
					addType();
				}}
			/>
		</div>
		{#if isAdmin}
			<p class="text-muted-foreground text-sm">{text.autoSave}</p>
		{:else}
			<p class="text-muted-foreground text-sm">{text.adminOnly}</p>
		{/if}
	</section>
{:else}
	<div class="rounded-lg border bg-muted/30 p-4 text-sm text-muted-foreground">
		{loadError}
	</div>
{/if}

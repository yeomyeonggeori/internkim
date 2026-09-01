<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { saveTaskDefinitions } from './task-api';
	import TaskDefinitionsView from './task-definitions-view.svelte';
	import type { LoadTask } from './task-load-tracker';
	import type { TaskDefinitions, TaskSizeDefinition, TaskSummary } from './task-types';
	import { taskDefinitionPaletteColor, unknownDefinitionColor } from './task-definition-colors';
	import { isCentralTaskSource } from './task-source';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskDefinitionsText = PageText<typeof taskText>['definitions'];
	type DefinitionSaveState = 'idle' | 'saving' | 'saved' | 'error';

	type Props = {
		summary: TaskSummary | null;
		loadError: string;
		text: TaskDefinitionsText;
		loadTask: LoadTask;
	};

	let { summary, loadError, text, loadTask }: Props = $props();

	let categoryDrafts = $state<string[]>([]);
	let typeDrafts = $state<string[]>([]);
	let sizeDrafts = $state<TaskSizeDefinition[]>([]);
	let categoryColorDrafts = $state<Record<string, string>>({});
	let typeColorDrafts = $state<Record<string, string>>({});
	let etcBusinessColorDraft = $state('');
	let etcTypeColorDraft = $state('');
	let newCategoryColor = $state('');
	let newTypeColor = $state('');
	let newCategoryText = $state('');
	let newTypeText = $state('');
	let isSavingDefinitions = $state(false);
	let definitionSaveState = $state<DefinitionSaveState>('idle');
	let definitionErrorMessage = $state('');

	const emptyDefinitions: TaskDefinitions = {
		categories: [],
		types: [],
		sizes: []
	};

	const definitions = () => summary?.definitions ?? emptyDefinitions;
	const currentWeek = () => summary?.week.code ?? '';
	const canEditDefinitions = () => summary !== null && definitions().sizes.length > 0;

	$effect(() => {
		const currentDefinitions = definitions();
		categoryDrafts = [...currentDefinitions.categories];
		typeDrafts = [...currentDefinitions.types];
		categoryColorDrafts = { ...(currentDefinitions.categoryColors ?? {}) };
		typeColorDrafts = { ...(currentDefinitions.typeColors ?? {}) };
		etcBusinessColorDraft = currentDefinitions.etcBusinessColor ?? '';
		etcTypeColorDraft = currentDefinitions.etcTypeColor ?? '';
		sizeDrafts = currentDefinitions.sizes.map((size) => ({ ...size }));
	});

	function addCategory(): void {
		const value = newCategoryText.trim();
		if (!value || categoryDrafts.includes(value)) return;
		categoryColorDrafts = { ...categoryColorDrafts, [value]: nextCategoryColor() };
		categoryDrafts = [...categoryDrafts, value];
		newCategoryText = '';
		newCategoryColor = '';
		void saveDefinitions();
	}

	function addType(): void {
		const value = newTypeText.trim();
		if (!value || typeDrafts.includes(value)) return;
		typeColorDrafts = { ...typeColorDrafts, [value]: nextTypeColor() };
		typeDrafts = [...typeDrafts, value];
		newTypeText = '';
		newTypeColor = '';
		void saveDefinitions();
	}

	function nextCategoryColor(): string {
		return newCategoryColor || taskDefinitionPaletteColor(categoryDrafts.length);
	}

	function nextTypeColor(): string {
		return newTypeColor || taskDefinitionPaletteColor(typeDrafts.length);
	}

	function setCategoryColor(index: number, color: string): void {
		const value = categoryDrafts[index];
		if (!value) return;
		categoryColorDrafts = { ...categoryColorDrafts, [value]: color };
		void saveDefinitions();
	}

	function setTypeColor(index: number, color: string): void {
		const value = typeDrafts[index];
		if (!value) return;
		typeColorDrafts = { ...typeColorDrafts, [value]: color };
		void saveDefinitions();
	}

	function setEtcBusinessColor(color: string): void {
		etcBusinessColorDraft = color;
		void saveDefinitions();
	}

	function setEtcTypeColor(color: string): void {
		etcTypeColorDraft = color;
		void saveDefinitions();
	}

	function updateCategory(index: number, value: string): void {
		categoryDrafts = categoryDrafts.map((item, itemIndex) => (itemIndex === index ? value : item));
	}

	function updateType(index: number, value: string): void {
		typeDrafts = typeDrafts.map((item, itemIndex) => (itemIndex === index ? value : item));
	}

	async function removeCategory(index: number): Promise<void> {
		categoryDrafts = categoryDrafts.filter((_item, itemIndex) => itemIndex !== index);
		await saveDefinitions();
	}

	async function removeType(index: number): Promise<void> {
		typeDrafts = typeDrafts.filter((_item, itemIndex) => itemIndex !== index);
		await saveDefinitions();
	}

	async function saveDefinitions(): Promise<void> {
		if (!summary?.isAdmin || !canEditDefinitions()) return;
		isSavingDefinitions = true;
		definitionSaveState = 'saving';
		definitionErrorMessage = '';
		try {
			const sizes = sizeDrafts.map((size) => ({
				...size,
				distanceKm: Math.max(1, Number(size.distanceKm) || 1),
				maxHours: Math.max(1, Number(size.maxHours) || 1),
				score: Math.max(1, Number(size.distanceKm) || 1),
				label: `${Math.max(1, Number(size.distanceKm) || 1)}km · ${Math.max(1, Number(size.maxHours) || 1)}h`
			}));
			await saveTaskDefinitions(
				{
					categories: categoryDrafts,
					categoryColors: colorsForValues(categoryDrafts, categoryColorDrafts),
					types: typeDrafts,
					typeColors: colorsForValues(typeDrafts, typeColorDrafts),
					...(etcBusinessColorDraft ? { etcBusinessColor: etcBusinessColorDraft } : {}),
					...(etcTypeColorDraft ? { etcTypeColor: etcTypeColorDraft } : {}),
					sizes
				},
				text.saveError,
				text.definitionInUse
			);
			await loadTask(currentWeek());
			definitionSaveState = 'saved';
			toast.success(text.saved);
		} catch (error) {
			definitionErrorMessage = error instanceof Error ? error.message : text.saveError;
			definitionSaveState = 'error';
			toast.error(definitionErrorMessage);
		} finally {
			isSavingDefinitions = false;
		}
	}

	function colorsForValues(values: string[], colors: Record<string, string>): Record<string, string> {
		return Object.fromEntries(values.filter((value) => colors[value]).map((value) => [value, colors[value]]));
	}

	function confirmRemoveCategory(index: number): void {
		const value = categoryDrafts[index];
		confirmDelete({
			title: text.removeBusinessTitle,
			description: text.removeBusinessDescription.replace('{value}', value),
			confirm: { text: text.removeAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await removeCategory(index);
			}
		});
	}

	function confirmRemoveType(index: number): void {
		const value = typeDrafts[index];
		confirmDelete({
			title: text.removeTypeTitle,
			description: text.removeTypeDescription.replace('{value}', value),
			confirm: { text: text.removeAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await removeType(index);
			}
		});
	}
</script>

<TaskDefinitionsView
	definitions={definitions()}
	{categoryDrafts}
	{setCategoryColor}
	newCategoryColor={nextCategoryColor()}
	newTypeColor={nextTypeColor()}
	setNewCategoryColor={(color) => (newCategoryColor = color)}
	setNewTypeColor={(color) => (newTypeColor = color)}
	{setTypeColor}
	{typeDrafts}
	{newCategoryText}
	{newTypeText}
	etcBusinessColor={etcBusinessColorDraft || unknownDefinitionColor}
	etcTypeColor={etcTypeColorDraft || unknownDefinitionColor}
	{setEtcBusinessColor}
	{setEtcTypeColor}
	canEditEtcColor={isCentralTaskSource(summary?.source ?? '')}
	isAdmin={summary?.isAdmin ?? false}
	canEditDefinitions={canEditDefinitions()}
	{isSavingDefinitions}
	{definitionSaveState}
	{definitionErrorMessage}
	{loadError}
	{text}
	{updateCategory}
	{updateType}
	removeCategory={confirmRemoveCategory}
	removeType={confirmRemoveType}
	{addCategory}
	{addType}
	setNewCategoryText={(value) => (newCategoryText = value)}
	setNewTypeText={(value) => (newTypeText = value)}
	{saveDefinitions}
/>

<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { saveTaskDefinitions } from './task-api';
	import TaskDefinitionsView from './task-definitions-view.svelte';
	import type { LoadTask } from './task-load-tracker';
	import type { TaskDefinitions, TaskSummary } from './task-types';
	import { unknownDefinitionColor } from './task-definition-colors';
	import { paletteColorAt } from '$lib/color-picker-palette';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskDefinitionsText = PageText<typeof taskText>['definitions'];
	type DefinitionSaveState = 'idle' | 'saving' | 'saved' | 'error';

	type Props = {
		summary: TaskSummary | null;
		isFresh: boolean;
		loadError: string;
		text: TaskDefinitionsText;
		loadTask: LoadTask;
	};

	let { summary, isFresh, loadError, text, loadTask }: Props = $props();

	let categoryDrafts = $state<string[]>([]);
	let typeDrafts = $state<string[]>([]);
	let categoryColorDrafts = $state<Record<string, string>>({});
	let typeColorDrafts = $state<Record<string, string>>({});
	let etcBusinessColorDraft = $state('');
	let etcTypeColorDraft = $state('');
	let newCategoryColor = $state('');
	let newTypeColor = $state('');
	let newCategoryText = $state('');
	let newTypeText = $state('');
	let definitionSaveState = $state<DefinitionSaveState>('idle');
	let definitionErrorMessage = $state('');

	const emptyDefinitions: TaskDefinitions = {
		categories: [],
		types: [],
		sizes: []
	};

	const definitions = () => summary?.definitions ?? emptyDefinitions;
	const currentWeek = () => summary?.week.code ?? '';
	const canEditDefinitions = () => isFresh && summary !== null && definitions().sizes.length > 0;

	$effect(() => {
		const currentDefinitions = definitions();
		categoryDrafts = [...currentDefinitions.categories];
		typeDrafts = [...currentDefinitions.types];
		categoryColorDrafts = { ...(currentDefinitions.categoryColors ?? {}) };
		typeColorDrafts = { ...(currentDefinitions.typeColors ?? {}) };
		etcBusinessColorDraft = currentDefinitions.etcBusinessColor ?? '';
		etcTypeColorDraft = currentDefinitions.etcTypeColor ?? '';
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
		return newCategoryColor || paletteColorAt(categoryDrafts.length);
	}

	function nextTypeColor(): string {
		return newTypeColor || paletteColorAt(typeDrafts.length);
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
		definitionSaveState = 'saving';
		definitionErrorMessage = '';
		try {
			await saveTaskDefinitions(
				{
					categories: categoryDrafts,
					categoryColors: colorsForValues(categoryDrafts, categoryColorDrafts),
					types: typeDrafts,
					typeColors: colorsForValues(typeDrafts, typeColorDrafts),
					...(etcBusinessColorDraft ? { etcBusinessColor: etcBusinessColorDraft } : {}),
					...(etcTypeColorDraft ? { etcTypeColor: etcTypeColorDraft } : {})
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
		}
	}

	function colorsForValues(values: string[], colors: Record<string, string>): Record<string, string> {
		return Object.fromEntries(values.filter((value) => colors[value]).map((value) => [value, colors[value]]));
	}

</script>

<TaskDefinitionsView
	definitions={definitions()}
	{categoryDrafts}
	{setCategoryColor}
	setNewCategoryColor={(color) => (newCategoryColor = color)}
	setNewTypeColor={(color) => (newTypeColor = color)}
	{setTypeColor}
	{typeDrafts}
	etcBusinessColor={etcBusinessColorDraft || unknownDefinitionColor}
	etcTypeColor={etcTypeColorDraft || unknownDefinitionColor}
	{setEtcBusinessColor}
	{setEtcTypeColor}
	isAdmin={summary?.isAdmin ?? false}
	hasDefinitions={summary !== null && definitions().sizes.length > 0}
	canEditDefinitions={canEditDefinitions()}
	{definitionSaveState}
	{loadError}
	{text}
	{updateCategory}
	{updateType}
	{removeCategory}
	{removeType}
	{addCategory}
	{addType}
	setNewCategoryText={(value) => (newCategoryText = value)}
	setNewTypeText={(value) => (newTypeText = value)}
	{saveDefinitions}
/>

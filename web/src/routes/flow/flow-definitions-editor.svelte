<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { saveFlowDefinitions } from './flow-api';
	import FlowDefinitionsView from './flow-definitions-view.svelte';
	import type { LoadFlow } from './flow-load-tracker';
	import type { FlowDefinitions, FlowSizeDefinition, FlowSummary } from './flow-types';
	import { flowDefinitionPaletteColor } from './flow-definition-colors';
	import { flowText } from './text';

	type FlowDefinitionsText = typeof flowText.ko.definitions;
	type DefinitionSaveState = 'idle' | 'saving' | 'saved' | 'error';

	type Props = {
		summary: FlowSummary | null;
		loadError: string;
		text: FlowDefinitionsText;
		loadFlow: LoadFlow;
	};

	let { summary, loadError, text, loadFlow }: Props = $props();

	let categoryDrafts = $state<string[]>([]);
	let typeDrafts = $state<string[]>([]);
	let sizeDrafts = $state<FlowSizeDefinition[]>([]);
	let categoryColorDrafts = $state<Record<string, string>>({});
	let typeColorDrafts = $state<Record<string, string>>({});
	let newCategoryColor = $state('');
	let newTypeColor = $state('');
	let newCategoryText = $state('');
	let newTypeText = $state('');
	let isSavingDefinitions = $state(false);
	let definitionSaveState = $state<DefinitionSaveState>('idle');
	let definitionErrorMessage = $state('');

	const emptyDefinitions: FlowDefinitions = {
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
		return newCategoryColor || flowDefinitionPaletteColor(categoryDrafts.length);
	}

	function nextTypeColor(): string {
		return newTypeColor || flowDefinitionPaletteColor(typeDrafts.length);
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
			await saveFlowDefinitions(
				{
					categories: categoryDrafts,
					categoryColors: colorsForValues(categoryDrafts, categoryColorDrafts),
					types: typeDrafts,
					typeColors: colorsForValues(typeDrafts, typeColorDrafts),
					sizes
				},
				text.saveError
			);
			await loadFlow(currentWeek());
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

<FlowDefinitionsView
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

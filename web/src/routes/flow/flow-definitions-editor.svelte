<script lang="ts">
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { saveFlowDefinitions } from './flow-api';
	import FlowDefinitionsView from './flow-definitions-view.svelte';
	import type { LoadFlow } from './flow-load-tracker';
	import type { FlowDefinitions, FlowSizeDefinition, FlowSummary } from './flow-types';
	import { flowText } from './text';

	type FlowDefinitionsText = typeof flowText.ko.definitions;

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
	let newCategoryText = $state('');
	let newTypeText = $state('');
	let isSavingDefinitions = $state(false);
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
		sizeDrafts = currentDefinitions.sizes.map((size) => ({ ...size }));
	});

	function addCategory(): void {
		const value = newCategoryText.trim();
		if (!value || categoryDrafts.includes(value)) return;
		categoryDrafts = [...categoryDrafts, value];
		newCategoryText = '';
	}

	function addType(): void {
		const value = newTypeText.trim();
		if (!value || typeDrafts.includes(value)) return;
		typeDrafts = [...typeDrafts, value];
		newTypeText = '';
	}

	function updateCategory(index: number, value: string): void {
		categoryDrafts = categoryDrafts.map((item, itemIndex) => (itemIndex === index ? value : item));
	}

	function updateType(index: number, value: string): void {
		typeDrafts = typeDrafts.map((item, itemIndex) => (itemIndex === index ? value : item));
	}

	function removeCategory(index: number): void {
		categoryDrafts = categoryDrafts.filter((_item, itemIndex) => itemIndex !== index);
	}

	function removeType(index: number): void {
		typeDrafts = typeDrafts.filter((_item, itemIndex) => itemIndex !== index);
	}

	async function saveDefinitions(): Promise<void> {
		if (!summary?.isAdmin || !canEditDefinitions()) return;
		isSavingDefinitions = true;
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
					types: typeDrafts,
					sizes
				},
				text.saveError
			);
			await loadFlow(currentWeek());
		} catch (error) {
			definitionErrorMessage = error instanceof Error ? error.message : text.saveError;
		} finally {
			isSavingDefinitions = false;
		}
	}

	function confirmRemoveCategory(index: number): void {
		const value = categoryDrafts[index];
		confirmDelete({
			title: text.removeBusinessTitle,
			description: text.removeBusinessDescription.replace('{value}', value),
			confirm: { text: text.removeAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				removeCategory(index);
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
				removeType(index);
			}
		});
	}
</script>

<FlowDefinitionsView
	definitions={definitions()}
	{categoryDrafts}
	{typeDrafts}
	{newCategoryText}
	{newTypeText}
	isAdmin={summary?.isAdmin ?? false}
	canEditDefinitions={canEditDefinitions()}
	{isSavingDefinitions}
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

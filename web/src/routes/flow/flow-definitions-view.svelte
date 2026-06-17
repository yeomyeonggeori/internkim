<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { flowProjectColor, flowTypeColor } from './flow-report-colors';
	import { sizeBadgeClass } from './flow-style';
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
		{@render SizeDefinitionCard()}
		<div class="grid gap-4 lg:grid-cols-2">
			{@render EditableListCard(
				text.business,
				text.businessDescription,
				categoryDrafts,
				newCategoryText,
				updateCategory,
				removeCategory,
				addCategory,
				setNewCategoryText,
				flowProjectColor
			)}
			{@render EditableListCard(
				text.type,
				text.typeDescription,
				typeDrafts,
				newTypeText,
				updateType,
				removeType,
				addType,
				setNewTypeText,
				flowTypeColor
			)}
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

{#snippet SizeDefinitionCard()}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.size}</Card.Title>
			<Card.Description>{text.sizeDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="overflow-hidden rounded-lg border">
				<Table.Root class="min-w-[980px]">
					<Table.Header class="bg-muted/40">
						<Table.Row class="hover:bg-transparent">
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.sizeName}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.distance}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.maxHours}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.developmentExample}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.otherExample}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.note}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each definitions.sizes as size (size.name)}
							<Table.Row>
								<Table.Cell><Badge class={sizeBadgeClass(size.name)}>{size.name}</Badge></Table.Cell>
								<Table.Cell class="text-right tabular-nums">{size.distanceKm}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{size.maxHours}</Table.Cell>
								<Table.Cell>{size.developmentExample}</Table.Cell>
								<Table.Cell>{size.otherExample}</Table.Cell>
								<Table.Cell class="text-muted-foreground">{size.note}</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		</Card.Content>
	</Card.Root>
{/snippet}

{#snippet EditableListCard(
	title: string,
	description: string,
	items: string[],
	newValue: string,
	update: (index: number, value: string) => void,
	remove: (index: number) => void,
	add: () => void,
	setNewValue: (value: string) => void,
	itemColor: (index: number) => string
)}
	<Card.Root size="sm">
		<Card.Header>
			<Card.Title>{title}</Card.Title>
			<Card.Description>{description}</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-2">
			{#each items as item, index}
				<div class="grid grid-cols-[auto_1fr_auto] items-center gap-2">
					<span class="size-2.5 rounded-full" style={`background: ${itemColor(index)}`}></span>
					<Input
						value={item}
						disabled={!isAdmin}
						oninput={(event) => update(index, event.currentTarget.value)}
						onblur={saveDefinitions}
					/>
					<Button
						variant="ghost"
						size="icon"
						disabled={!isAdmin}
						onclick={() => remove(index)}
						aria-label={text.removeAction}
					>
						<Trash2Icon class="size-4" />
					</Button>
				</div>
			{/each}
			{#if isAdmin}
				<div class="grid grid-cols-[auto_1fr_auto] items-center gap-2">
					<span class="size-2.5 rounded-full" style={`background: ${itemColor(items.length)}`}></span>
					<Input value={newValue} placeholder={title} oninput={(event) => setNewValue(event.currentTarget.value)} />
					<Button variant="outline" size="icon" onclick={add} aria-label={text.add}>
						<PlusIcon class="size-4" />
					</Button>
				</div>
			{/if}
			{#if items.length === 0 && !isAdmin}
				<p class="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">—</p>
			{/if}
		</Card.Content>
	</Card.Root>
{/snippet}

<script lang="ts">
	import ColorPicker from '$lib/components/color-picker.svelte';
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { fallbackPickerColor, paletteColorAt } from '$lib/color-picker-palette';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';

	export type DefinitionListItem = {
		id: string;
		name: string;
		color?: string;
	};

	type Props = {
		title: string;
		items: DefinitionListItem[];
		itemColor?: (item: DefinitionListItem, index: number) => string;
		addLabel: string;
		removeLabel: string;
		removeTitle: string;
		removeDescription: string;
		cancelLabel: string;
		colorLabel: string;
		doneLabel: string;
		emptyLabel: string;
		saveState?: string;
		isEditable?: boolean;
		fallbackItem?: { name: string; color: string };
		onRename: (id: string, name: string) => void;
		onColorChange: (id: string, color: string) => void;
		onRemove: (id: string) => void;
		onAdd: (name: string, color: string) => void;
		onFallbackColorChange?: (color: string) => void;
	};

	let {
		title,
		items,
		itemColor,
		addLabel,
		removeLabel,
		removeTitle,
		removeDescription,
		cancelLabel,
		colorLabel,
		doneLabel,
		emptyLabel,
		saveState = '',
		isEditable = true,
		fallbackItem,
		onRename,
		onColorChange,
		onRemove,
		onAdd,
		onFallbackColorChange
	}: Props = $props();

	const rowClass = 'flex h-9 items-center gap-3 rounded-md px-2 hover:bg-muted/40';
	const nameInputClass =
		'h-7 border-transparent bg-transparent px-2 shadow-none dark:bg-transparent focus-visible:bg-background focus-visible:ring-1 focus-visible:ring-ring/40 focus-visible:border-transparent dark:focus-visible:bg-input/30';

	let isAddingItem = $state(false);
	let addColor = $state(fallbackPickerColor);
	let addInput = $state<HTMLInputElement | null>(null);

	function colorOf(item: DefinitionListItem, index: number): string {
		if (itemColor) return itemColor(item, index);
		return item.color ?? fallbackPickerColor;
	}

	function commitName(input: HTMLInputElement, item: DefinitionListItem): void {
		const name = input.value.trim();
		if (!name || name === item.name) {
			input.value = item.name;
			return;
		}
		onRename(item.id, name);
	}

	function handleNameKeydown(
		event: KeyboardEvent & { currentTarget: HTMLInputElement },
		item: DefinitionListItem
	): void {
		if (event.key === 'Enter') event.currentTarget.blur();
		if (event.key !== 'Escape') return;
		event.currentTarget.value = item.name;
		event.currentTarget.blur();
	}

	function requestRemove(item: DefinitionListItem): void {
		confirmDelete({
			title: removeTitle,
			description: removeDescription.replace('{name}', item.name),
			confirm: { text: removeLabel },
			cancel: { text: cancelLabel },
			onConfirm: async () => onRemove(item.id)
		});
	}

	function startAdding(): void {
		addColor = paletteColorAt(items.length);
		isAddingItem = true;
	}

	function stopAdding(): void {
		isAddingItem = false;
	}

	function commitNewName(): void {
		const name = addInput?.value.trim() ?? '';
		if (!name) return stopAdding();
		onAdd(name, addColor);
		stopAdding();
	}

	function handleAddKeydown(event: KeyboardEvent): void {
		if (event.key === 'Enter') commitNewName();
		if (event.key === 'Escape') stopAdding();
	}

	$effect(() => {
		if (isAddingItem) addInput?.focus();
	});
</script>

<Card.Root size="sm">
	<Card.Header>
		<div class="flex items-baseline justify-between gap-2">
			<Card.Title>{title}</Card.Title>
			{#if saveState}<span class="text-muted-foreground text-xs">{saveState}</span>{/if}
		</div>
	</Card.Header>
	<Card.Content>
		<div class="space-y-1">
			{#each items as item, index (item.id)}
				<div class={rowClass}>
					{#if isEditable}
						<ColorPicker
							value={colorOf(item, index)}
							label={colorLabel} {doneLabel}
							onChange={(color) => onColorChange(item.id, color)}
						/>
					{:else}
						<span class="ring-border size-4 shrink-0 rounded-sm ring-1" style={`background: ${colorOf(item, index)}`}
						></span>
					{/if}
					{#if isEditable}
						<Input
							value={item.name}
							aria-label={item.name}
							class={nameInputClass}
							onkeydown={(event) => handleNameKeydown(event, item)}
							onblur={(event) => commitName(event.currentTarget, item)}
						/>
						<Tooltip.Root>
							<Tooltip.Trigger>
								{#snippet child({ props })}
									<Button
										{...props}
										variant="ghost"
										size="icon-sm"
										class="text-muted-foreground hover:text-destructive"
										aria-label={removeLabel}
										onclick={() => requestRemove(item)}
									>
										<Trash2Icon class="size-4" />
									</Button>
								{/snippet}
							</Tooltip.Trigger>
							<Tooltip.Content>{removeLabel}</Tooltip.Content>
						</Tooltip.Root>
					{:else}
						<span class="px-2 text-sm">{item.name}</span>
					{/if}
				</div>
			{/each}

			{#if fallbackItem}
				<div class={rowClass}>
					{#if isEditable && onFallbackColorChange}
						<ColorPicker value={fallbackItem.color} label={colorLabel} {doneLabel} onChange={onFallbackColorChange} />
					{:else}
						<span class="ring-border size-4 shrink-0 rounded-sm ring-1" style={`background: ${fallbackItem.color}`}
						></span>
					{/if}
					<span class="text-muted-foreground px-2 text-sm">{fallbackItem.name}</span>
				</div>
			{/if}

			{#if items.length === 0 && !fallbackItem}
				<Empty.Root class="p-2"><Empty.Header><Empty.Title>{emptyLabel}</Empty.Title></Empty.Header></Empty.Root>
			{/if}
		</div>

		{#if isEditable}
			{#if isAddingItem}
				<div class={`${rowClass} mt-1`}>
					<ColorPicker value={addColor} label={colorLabel} {doneLabel} onChange={(color) => (addColor = color)} />
					<Input
						bind:ref={addInput}
						placeholder={title}
						class={nameInputClass}
						onkeydown={handleAddKeydown}
					/>
					<Button variant="ghost" size="icon-sm" aria-label={addLabel} onclick={commitNewName}>
						<PlusIcon class="size-4" />
					</Button>
				</div>
			{:else}
				<Button variant="ghost" size="sm" class="text-muted-foreground mt-1 h-8 px-2" onclick={startAdding}>
					<PlusIcon class="size-4" />
					{addLabel}
				</Button>
			{/if}
		{/if}
	</Card.Content>
</Card.Root>

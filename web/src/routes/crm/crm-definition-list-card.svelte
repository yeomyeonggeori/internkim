<script lang="ts">
	import ColorPicker from '$lib/components/color-picker.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import type { CRMDefinition } from './crm-definitions';

	type Props = {
		title: string;
		description: string;
		items: CRMDefinition[];
		isAdmin: boolean;
		disabled: boolean;
		addLabel: string;
		removeLabel: string;
		colorLabel: string;
		onNameInput: (id: string, name: string) => void;
		onCommit: () => void;
		onColorChange: (id: string, color: string) => void;
		onRemove: (id: string) => void;
		onAdd: (name: string, color: string) => void;
	};

	let {
		title,
		description,
		items,
		isAdmin,
		disabled,
		addLabel,
		removeLabel,
		colorLabel,
		onNameInput,
		onCommit,
		onColorChange,
		onRemove,
		onAdd
	}: Props = $props();

	let newName = $state('');
	let newColor = $state('#64748b');

	function add(): void {
		const name = newName.trim();
		if (!name) return;
		onAdd(name, newColor);
		newName = '';
	}
</script>

<Card.Root size="sm">
	<Card.Header>
		<Card.Title>{title}</Card.Title>
		<Card.Description>{description}</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-2">
		{#each items as item (item.id)}
			<div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2">
				{#if isAdmin && !disabled}
					<ColorPicker
						value={item.color ?? '#64748b'}
						label={colorLabel}
						class="size-7"
						onChange={(color) => onColorChange(item.id, color)}
					/>
				{:else}
					<span class="size-2.5 rounded-full" style={`background: ${item.color ?? '#64748b'}`}></span>
				{/if}
				<Input
					value={item.name}
					disabled={!isAdmin || disabled}
					oninput={(event) => onNameInput(item.id, event.currentTarget.value)}
					onblur={onCommit}
				/>
				<Button
					variant="ghost"
					size="icon"
					disabled={!isAdmin || disabled}
					onclick={() => onRemove(item.id)}
					aria-label={removeLabel}
				>
					<Trash2Icon class="size-4" />
				</Button>
			</div>
		{/each}
		{#if isAdmin}
			<div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2">
				<ColorPicker value={newColor} label={colorLabel} class="size-7" onChange={(color) => (newColor = color)} />
				<Input
					value={newName}
					disabled={disabled}
					placeholder={title}
					oninput={(event) => (newName = event.currentTarget.value)}
					onkeydown={(event) => event.key === 'Enter' && add()}
				/>
				<Button variant="outline" size="icon" disabled={disabled || !newName.trim()} onclick={add} aria-label={addLabel}>
					<PlusIcon class="size-4" />
				</Button>
			</div>
		{/if}
		{#if items.length === 0 && !isAdmin}
			<p class="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">—</p>
		{/if}
	</Card.Content>
</Card.Root>

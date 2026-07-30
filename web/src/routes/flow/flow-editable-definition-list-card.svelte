<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import ColorPicker from '$lib/components/color-picker.svelte';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';

	type Props = {
		title: string;
		description: string;
		items: string[];
		newValue: string;
		isAdmin: boolean;
		removeLabel: string;
		addLabel: string;
		update: (index: number, value: string) => void;
		remove: (index: number) => void;
		add: () => void;
		setNewValue: (value: string) => void;
		saveDefinitions: () => void;
		itemColor: (index: number) => string;
		setItemColor: (index: number, color: string) => void;
		newColor: string;
		setNewColor: (color: string) => void;
		colorLabel: string;
	};

	let {
		title,
		description,
		items,
		newValue,
		isAdmin,
		removeLabel,
		addLabel,
		update,
		remove,
		add,
		setNewValue,
		saveDefinitions,
		itemColor,
		setItemColor,
		newColor,
		setNewColor,
		colorLabel
	}: Props = $props();
</script>

<Card.Root size="sm">
	<Card.Header>
		<Card.Title>{title}</Card.Title>
		<Card.Description>{description}</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-2">
		{#each items as item, index}
			<div class="grid grid-cols-[auto_1fr_auto] items-center gap-2">
				{#if isAdmin}
					<ColorPicker
						value={itemColor(index)}
						label={colorLabel}
						class="size-7"
						onChange={(color) => setItemColor(index, color)}
					/>
				{:else}
					<span class="size-2.5 rounded-full" style={`background: ${itemColor(index)}`}></span>
				{/if}
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
					aria-label={removeLabel}
				>
					<Trash2Icon class="size-4" />
				</Button>
			</div>
		{/each}
		{#if isAdmin}
			<div class="grid grid-cols-[auto_1fr_auto] items-center gap-2">
				<ColorPicker value={newColor} label={colorLabel} class="size-7" onChange={setNewColor} />
				<Input value={newValue} placeholder={title} oninput={(event) => setNewValue(event.currentTarget.value)} />
				<Button variant="outline" size="icon" onclick={add} aria-label={addLabel}>
					<PlusIcon class="size-4" />
				</Button>
			</div>
		{/if}
		{#if items.length === 0 && !isAdmin}
			<p class="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">—</p>
		{/if}
	</Card.Content>
</Card.Root>

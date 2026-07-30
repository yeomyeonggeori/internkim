<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import * as Popover from '$lib/components/ui/popover';
	import { cn } from '$lib/utils';
	import { colorPickerPalette, normalizeColor } from '$lib/color-picker-palette';

	type Props = {
		value: string;
		label: string;
		customLabel?: string;
		onChange: (color: string) => void;
		class?: string;
	};

	let { value, label, customLabel = label, onChange, class: className }: Props = $props();

	let isOpen = $state(false);
	const selectedColor = $derived(normalizeColor(value));

	function selectColor(color: string): void {
		onChange(color);
		isOpen = false;
	}
</script>

<Popover.Root bind:open={isOpen}>
	<Popover.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				aria-label={label}
				class={cn('border-border/70 size-8 shrink-0 rounded-md border shadow-none', className)}
				style={`background: ${selectedColor}`}
			></button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-auto p-2" align="start">
		<div class="grid grid-cols-6 gap-1.5">
			{#each colorPickerPalette as paletteColor (paletteColor)}
				<button
					type="button"
					aria-label={paletteColor}
					aria-pressed={paletteColor === selectedColor}
					class="flex size-7 items-center justify-center rounded-md"
					style={`background: ${paletteColor}`}
					onclick={() => selectColor(paletteColor)}
				>
					{#if paletteColor === selectedColor}
						<CheckIcon class="size-4 text-white" />
					{/if}
				</button>
			{/each}
		</div>
		<label class="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
			<input
				type="color"
				value={selectedColor}
				aria-label={customLabel}
				class="bg-background size-7 rounded-md border"
				oninput={(event) => onChange(event.currentTarget.value)}
			/>
			{customLabel}
		</label>
	</Popover.Content>
</Popover.Root>

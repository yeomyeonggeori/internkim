<script lang="ts">
	import * as Drawer from '$lib/components/ui/drawer';
	import * as Popover from '$lib/components/ui/popover';
	import { Button } from '$lib/components/ui/button';
	import { Slider } from '$lib/components/ui/slider';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { cn } from '$lib/utils';
	import { hexToHSL, hslToHex } from '$lib/color-hsl';
	import { colorPickerPalette, normalizeColor, paletteSaturation } from '$lib/color-picker-palette';

	type Props = {
		value: string;
		label: string;
		hueLabel?: string;
		lightnessLabel?: string;
		doneLabel?: string;
		onChange: (color: string) => void;
		class?: string;
	};

	let {
		value,
		label,
		hueLabel = 'Hue',
		lightnessLabel = 'Lightness',
		doneLabel = 'Done',
		onChange,
		class: className
	}: Props = $props();

	const columnCount = 8;
	const trackClass =
		'[&_[data-slot=slider-range]]:bg-transparent [&_[data-slot=slider-track]]:bg-[image:var(--slider-track)]';
	const lowestLightness = 20;
	const highestLightness = 80;

	const isMobile = new IsMobile();

	let isOpen = $state(false);
	let focusedIndex = $state(0);
	let swatchButtons: HTMLButtonElement[] = $state([]);
	const selectedColor = $derived(normalizeColor(value));
	const selected = $derived(hexToHSL(selectedColor));
	const hueTrack =
		'linear-gradient(to right, ' +
		[0, 60, 120, 180, 240, 300, 360].map((hue) => hslToHex({ hue, saturation: paletteSaturation, lightness: 50 })).join(', ') +
		')';
	const lightnessTrack = $derived(
		`linear-gradient(to right, ${hslToHex({ ...selected, saturation: paletteSaturation, lightness: lowestLightness })}, ${hslToHex({ ...selected, saturation: paletteSaturation, lightness: highestLightness })})`
	);

	function changeHue(hue: number): void {
		onChange(hslToHex({ hue, saturation: paletteSaturation, lightness: selected.lightness }));
	}

	function changeLightness(lightness: number): void {
		onChange(hslToHex({ hue: selected.hue, saturation: paletteSaturation, lightness }));
	}

	function focusSwatch(index: number): void {
		const boundedIndex = Math.min(Math.max(index, 0), colorPickerPalette.length - 1);
		focusedIndex = boundedIndex;
		swatchButtons[boundedIndex]?.focus();
	}

	function moveFocus(event: KeyboardEvent, index: number): void {
		const step = {
			ArrowRight: 1,
			ArrowLeft: -1,
			ArrowDown: columnCount,
			ArrowUp: -columnCount
		}[event.key];
		if (step === undefined) return;
		event.preventDefault();
		focusSwatch(index + step);
	}

	$effect(() => {
		if (!isOpen) return;
		const selectedIndex = colorPickerPalette.indexOf(selectedColor as (typeof colorPickerPalette)[number]);
		focusedIndex = selectedIndex < 0 ? 0 : selectedIndex;
	});
</script>

{#snippet swatchTrigger(triggerProps: Record<string, unknown>)}
	<button
		{...triggerProps}
		type="button"
		aria-label={label}
		class={cn('ring-border size-4 shrink-0 rounded-sm ring-1', className)}
		style={`background: ${selectedColor}`}
	></button>
{/snippet}

{#snippet sliders()}
	<Slider
		type="single"
		size="lg"
		value={selected.hue}
		min={0}
		max={360}
		step={1}
		aria-label={hueLabel}
		class={trackClass}
		style={`--slider-track: ${hueTrack}; --slider-thumb-border: ${selectedColor}`}
		onValueChange={changeHue}
	/>
	<Slider
		type="single"
		size="lg"
		value={selected.lightness}
		min={lowestLightness}
		max={highestLightness}
		step={1}
		aria-label={lightnessLabel}
		class={trackClass}
		style={`--slider-track: ${lightnessTrack}; --slider-thumb-border: ${selectedColor}`}
		onValueChange={changeLightness}
	/>
{/snippet}

{#snippet palette(gridGapClass: string, swatchShapeClass: string)}
	<div class={cn('grid grid-cols-8', gridGapClass)}>
		{#each colorPickerPalette as paletteColor, index (paletteColor)}
			<button
				bind:this={swatchButtons[index]}
				type="button"
				aria-label={paletteColor}
				aria-pressed={paletteColor === selectedColor}
				tabindex={index === focusedIndex ? 0 : -1}
				class={cn(
					'rounded-md border-0 outline-none ring-offset-0 transition hover:scale-110 focus-visible:ring-3 focus-visible:ring-ring/50',
					swatchShapeClass,
					paletteColor === selectedColor && 'ring-ring/50 ring-3'
				)}
				style={`background: ${paletteColor}`}
				onclick={() => onChange(paletteColor)}
				onkeydown={(event) => moveFocus(event, index)}
			></button>
		{/each}
	</div>
{/snippet}

{#if isMobile.current}
	<Drawer.Root bind:open={isOpen}>
		<Drawer.Trigger>
			{#snippet child({ props })}
				{@render swatchTrigger(props)}
			{/snippet}
		</Drawer.Trigger>
		<Drawer.Content class="pb-[max(1rem,env(safe-area-inset-bottom))]">
			<Drawer.Header class="items-start">
				<Drawer.Title class="sr-only">{label}</Drawer.Title>
				<span class="ring-border size-14 rounded-md ring-1" style={`background: ${selectedColor}`}></span>
			</Drawer.Header>
			<div class="grid gap-4 px-4 pb-4">
				{@render sliders()}
			</div>
			<div class="px-4 pb-4">
				{@render palette('gap-3', 'h-7 w-full')}
			</div>
			<Drawer.Footer class="pt-0">
				<Drawer.Close>
					{#snippet child({ props })}
						<Button {...props} variant="outline">{doneLabel}</Button>
					{/snippet}
				</Drawer.Close>
			</Drawer.Footer>
		</Drawer.Content>
	</Drawer.Root>
{:else}
	<Popover.Root bind:open={isOpen}>
		<Popover.Trigger>
			{#snippet child({ props })}
				{@render swatchTrigger(props)}
			{/snippet}
		</Popover.Trigger>
		<Popover.Content class="w-72 space-y-4 p-3" align="start">
			<div class="grid grid-cols-[auto_1fr] items-center gap-3">
				<span class="ring-border size-14 rounded-md ring-1" style={`background: ${selectedColor}`}></span>
				<div class="grid gap-2">
					{@render sliders()}
				</div>
			</div>
			{@render palette('gap-1.5', 'size-6')}
		</Popover.Content>
	</Popover.Root>
{/if}

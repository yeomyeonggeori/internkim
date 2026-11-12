<script lang="ts">
	import Minus from '@lucide/svelte/icons/minus';
	import Plus from '@lucide/svelte/icons/plus';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';

	type LeaveDayCountInputProps = {
		id: string;
		value: number | null;
		step?: number;
		min?: number;
		max?: number;
		unit: string;
		placeholder?: string;
		disabled?: boolean;
		invalid?: boolean;
		increaseLabel: string;
		decreaseLabel: string;
		onChange: (value: number | null) => void;
	};

	let {
		id,
		value,
		step = 0.25,
		min = 0,
		max,
		unit,
		placeholder,
		disabled = false,
		invalid = false,
		increaseLabel,
		decreaseLabel,
		onChange
	}: LeaveDayCountInputProps = $props();

	const isMobile = new IsMobile();

	const decimals = $derived(String(step).split('.')[1]?.length ?? 0);
	const atLeast = $derived(value === null ? min : value);
	const canDecrease = $derived(!disabled && value !== null && value > min);
	const canIncrease = $derived(!disabled && (max === undefined || atLeast < max));

	function rounded(next: number): number {
		return Number(next.toFixed(decimals));
	}

	function within(next: number): number {
		const notBelow = Math.max(min, next);
		return max === undefined ? notBelow : Math.min(max, notBelow);
	}

	function stepBy(direction: number): void {
		onChange(rounded(within(atLeast + direction * step)));
	}

	function written(typed: string): void {
		const trimmed = typed.trim();
		if (!trimmed) return onChange(null);
		const asNumber = Number(trimmed);
		onChange(Number.isFinite(asNumber) ? rounded(within(asNumber)) : null);
	}
</script>

<div class="flex min-w-0 items-start gap-2">
	<InputGroup.Root class="min-w-0 flex-1" data-invalid={invalid || undefined}>
		<InputGroup.Input
			{id}
			type="number"
			inputmode="decimal"
			class="no-spin"
			min={min}
			max={max}
			step={step}
			value={value ?? ''}
			{placeholder}
			{disabled}
			aria-invalid={invalid}
			oninput={(event) => written(event.currentTarget.value)}
		/>
		<InputGroup.Addon align="inline-end">
			<InputGroup.Text>{unit}</InputGroup.Text>
		</InputGroup.Addon>
	</InputGroup.Root>
	<ButtonGroup.Root orientation={isMobile.current ? 'horizontal' : 'vertical'} class="h-fit shrink-0">
		<Button
			type="button"
			variant="outline"
			size="icon"
			class={isMobile.current ? '' : 'size-4 rounded-b-none'}
			aria-label={increaseLabel}
			aria-controls={id}
			disabled={!canIncrease}
			onclick={() => stepBy(1)}
		>
			<Plus />
		</Button>
		<Button
			type="button"
			variant="outline"
			size="icon"
			class={isMobile.current ? '' : 'size-4 rounded-t-none'}
			aria-label={decreaseLabel}
			aria-controls={id}
			disabled={!canDecrease}
			onclick={() => stepBy(-1)}
		>
			<Minus />
		</Button>
	</ButtonGroup.Root>
</div>

<style>
	:global(.no-spin::-webkit-outer-spin-button),
	:global(.no-spin::-webkit-inner-spin-button) {
		appearance: none;
		margin: 0;
	}

	:global(.no-spin) {
		appearance: textfield;
	}
</style>

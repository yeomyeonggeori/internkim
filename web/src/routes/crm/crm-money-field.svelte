<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { formatAmountInput } from './crm-money';
	import { findCurrencyCatalogueEntry, type CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { currencyDisplayNamesFor, currencyNameOf } from '$lib/currency/currency-name';
	import type { CRMCurrency } from './crm-types';

	type Props = {
		id: string;
		label: string;
		currencyLabel: string;
		value: string;
		currency: CRMCurrency;
		currencyCatalogue: CurrencyCatalogue;
		disabled?: boolean;
	};

	let { id, label, currencyLabel, value = $bindable(), currency = $bindable(), currencyCatalogue, disabled = false }: Props = $props();

	function updateAmount(event: Event & { currentTarget: HTMLInputElement }): void {
		value = formatAmountInput(event.currentTarget.value);
	}

	function updateCurrency(value: string): void {
		if (findCurrencyCatalogueEntry(currencyCatalogue, value)) currency = value;
	}

	const currencyDisplayNames = $derived(currencyDisplayNamesFor(currentLocale.value));
</script>

<Field.Field>
	<Field.Label for={id}>{label}</Field.Label>
	<div class="grid grid-cols-[6.5rem_minmax(0,1fr)] gap-2">
		<Select.Root type="single" value={currency} onValueChange={updateCurrency} {disabled}>
			<Select.Trigger id={`${id}-currency`} class="w-full" aria-label={currencyLabel}>{currency}</Select.Trigger>
			<Select.Content class="max-h-72 w-fit"><Select.Group>
				{#each currencyCatalogue as option (option.code)}
					<Select.Item value={option.code} label={`${option.code} ${currencyNameOf(option, currencyDisplayNames)}`}>
						<span class="w-9 shrink-0 font-medium">{option.code}</span>
						<span class="truncate text-muted-foreground">{currencyNameOf(option, currencyDisplayNames)}</span>
					</Select.Item>
				{/each}
			</Select.Group></Select.Content>
		</Select.Root>
		<div class="relative">
			<Input
				{id}
				type="text"
				inputmode="numeric"
				autocomplete="off"
				value={value}
				oninput={updateAmount}
				{disabled}
			/>
		</div>
	</div>
</Field.Field>

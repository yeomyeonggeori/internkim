<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import { toast } from 'svelte-sonner';
	import type { CurrencyCatalogue, CurrencyCatalogueEntry } from '$lib/currency/currency-catalogue';
	import { crmViewCurrency, originalViewCurrency } from './crm-view-currency.svelte';
	import type { CRMText } from './text';

	type Props = {
		text: CRMText;
		currencyCatalogue: CurrencyCatalogue;
		sourceCurrencies: string[];
	};

	let { text, currencyCatalogue, sourceCurrencies }: Props = $props();

	const chosenEntry = $derived(currencyCatalogue.find((entry) => entry.code === crmViewCurrency.selected));

	const currencyDisplayNames = $derived(
		new Intl.DisplayNames([currentLocale.value === 'ko' ? 'ko' : 'en'], { type: 'currency' })
	);

	function distinctSymbolOf(entry: CurrencyCatalogueEntry): string {
		return entry.symbol === entry.code ? '' : entry.symbol;
	}

	function currencyNameOf(entry: CurrencyCatalogueEntry): string {
		try {
			return currencyDisplayNames.of(entry.code) ?? entry.name;
		} catch {
			return entry.name;
		}
	}

	function triggerLabelOf(entry: CurrencyCatalogueEntry): string {
		const symbol = distinctSymbolOf(entry);
		return symbol ? `${symbol} ${entry.code}` : entry.code;
	}

	async function handleChange(value: string): Promise<void> {
		const succeeded = await crmViewCurrency.choose(value, sourceCurrencies);
		if (!succeeded) toast.error(text.viewCurrencyFailed);
	}
</script>

{#snippet currencyRow(entry: CurrencyCatalogueEntry)}
	<span class="w-9 shrink-0 text-muted-foreground">{distinctSymbolOf(entry)}</span>
	<span class="w-12 shrink-0 font-medium">{entry.code}</span>
	<span class="truncate">{currencyNameOf(entry)}</span>
{/snippet}

{#if isSupabaseConfigured()}
	<Select.Root
		type="single"
		value={crmViewCurrency.selected}
		onValueChange={(value) => void handleChange(value)}
		disabled={crmViewCurrency.isLoading}
	>
		<Select.Trigger class="w-28 shrink-0 sm:w-32" aria-label={text.viewCurrency}>
			{chosenEntry ? triggerLabelOf(chosenEntry) : text.viewCurrencyOriginal}
		</Select.Trigger>
		<Select.Content class="max-h-72 [mask-image:linear-gradient(to_bottom,black_calc(100%-2.5rem),transparent)]">
			<Select.Item value={originalViewCurrency} label={text.viewCurrencyOriginal}>{text.viewCurrencyOriginal}</Select.Item>
			{#each currencyCatalogue as option (option.code)}
				<Select.Item value={option.code} label={`${triggerLabelOf(option)} ${currencyNameOf(option)}`}>
					{@render currencyRow(option)}
				</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/if}

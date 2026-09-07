<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { currencyDisplayNamesFor, currencyNameOf } from '$lib/currency/currency-name';
	import { toast } from 'svelte-sonner';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import { crmViewCurrency, isViewCurrencyAvailable } from './crm-view-currency.svelte';
	import { formatViewRateHint } from './crm-money';
	import type { CRMText } from './text';

	type Props = {
		text: CRMText;
		currencyCatalogue: CurrencyCatalogue;
		companyBaseCurrency: string;
		sourceCurrencies: string[];
	};

	let { text, currencyCatalogue, companyBaseCurrency, sourceCurrencies }: Props = $props();

	const currencyDisplayNames = $derived(currencyDisplayNamesFor(currentLocale.value));

	const rateHint = $derived(
		formatViewRateHint(crmViewCurrency.selected, crmViewCurrency.ratesBySource)
	);

	async function handleChange(value: string): Promise<void> {
		const succeeded = await crmViewCurrency.choose(value, sourceCurrencies);
		if (!succeeded) toast.error(text.viewCurrencyFailed);
	}
</script>

{#if isViewCurrencyAvailable()}
	<Select.Root
		type="single"
		value={crmViewCurrency.selected}
		onValueChange={handleChange}
		disabled={crmViewCurrency.isLoading}
	>
		<Select.Trigger class="shrink-0" aria-label={text.viewCurrency}>
			{crmViewCurrency.selected || companyBaseCurrency || text.viewCurrency}
		</Select.Trigger>
		<Select.Content class="max-h-72">
			{#each currencyCatalogue as option (option.code)}
				<Select.Item value={option.code} label={`${option.code} ${currencyNameOf(option, currencyDisplayNames)}`}>
					<span class="w-9 shrink-0 font-medium">{option.code}</span>
					<span class="truncate text-muted-foreground">{currencyNameOf(option, currencyDisplayNames)}</span>
				</Select.Item>
			{/each}
			{#if rateHint !== ''}
				<p class="mt-1 border-t px-2 pb-1 pt-2 text-xs text-muted-foreground">{rateHint}</p>
			{/if}
		</Select.Content>
	</Select.Root>
{/if}

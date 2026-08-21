<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { currencyDisplayNamesFor, currencyNameOf } from '$lib/currency/currency-name';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import { toast } from 'svelte-sonner';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import { crmViewCurrency } from './crm-view-currency.svelte';
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

	$effect(() => {
		if (!isSupabaseConfigured()) return;
		void crmViewCurrency.follow(companyBaseCurrency, sourceCurrencies);
	});

	async function handleChange(value: string): Promise<void> {
		const succeeded = await crmViewCurrency.choose(value, sourceCurrencies);
		if (!succeeded) toast.error(text.viewCurrencyFailed);
	}
</script>

{#if isSupabaseConfigured()}
	<Select.Root
		type="single"
		value={crmViewCurrency.selected}
		onValueChange={handleChange}
		disabled={crmViewCurrency.isLoading}
	>
		<Select.Trigger class="shrink-0" aria-label={text.viewCurrency}>
			{crmViewCurrency.selected || companyBaseCurrency || text.viewCurrency}
			{#if rateHint !== ''}
				<span class="text-xs whitespace-nowrap text-muted-foreground">{rateHint}</span>
			{/if}
		</Select.Trigger>
		<Select.Content class="max-h-72">
			{#each currencyCatalogue as option (option.code)}
				<Select.Item value={option.code} label={`${option.code} ${currencyNameOf(option, currencyDisplayNames)}`}>
					<span class="w-12 shrink-0 font-medium">{option.code}</span>
					<span class="truncate text-muted-foreground">{currencyNameOf(option, currencyDisplayNames)}</span>
				</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/if}

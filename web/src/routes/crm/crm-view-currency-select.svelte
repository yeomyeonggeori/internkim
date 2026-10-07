<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { currencyDisplayNamesFor, currencyNameOf } from '$lib/currency/currency-name';
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

	let { text, currencyCatalogue, sourceCurrencies }: Props = $props();

	const currencyDisplayNames = $derived(currencyDisplayNamesFor(currentLocale.value));

	const rateHint = $derived(
		formatViewRateHint(crmViewCurrency.selected, crmViewCurrency.ratesBySource)
	);

	async function handleChange(value: string): Promise<void> {
		await crmViewCurrency.choose(value, sourceCurrencies);
	}
</script>

{#if isViewCurrencyAvailable()}
	<div class="flex flex-wrap items-center gap-2" data-crm-view-currency>
	<Select.Root
		type="single"
		bind:value={() => crmViewCurrency.selected, (value) => { void handleChange(value); }}
	>
		<Select.Trigger class="shrink-0" aria-label={text.viewCurrency} aria-busy={crmViewCurrency.isLoading}>
			{crmViewCurrency.selected || text.viewCurrencyOriginal}
		</Select.Trigger>
		<Select.Content class="max-h-72"><Select.Group>
			{#each currencyCatalogue as option (option.code)}
				<Select.Item value={option.code} label={`${option.code} ${currencyNameOf(option, currencyDisplayNames)}`}>
					<span class="w-9 shrink-0 font-medium">{option.code}</span>
					<span class="truncate text-muted-foreground">{currencyNameOf(option, currencyDisplayNames)}</span>
				</Select.Item>
			{/each}
			{#if rateHint !== ''}
				<p class="mt-1 border-t px-2 pb-1 pt-2 text-xs text-muted-foreground">{rateHint}</p>
			{/if}
		</Select.Group></Select.Content>
	</Select.Root>
	{#if crmViewCurrency.isLoading}
		<span role="status" class="flex items-center gap-1 text-xs text-muted-foreground">
			<LoaderCircle class="size-3 motion-safe:animate-spin" aria-hidden="true" />
			{text.viewCurrencyLoading.replace('{currency}', crmViewCurrency.pending)}
		</span>
		{#if crmViewCurrency.selected && crmViewCurrency.pending !== crmViewCurrency.selected}
			<Button variant="ghost" size="sm" onclick={() => handleChange(crmViewCurrency.selected)}>{text.cancel}</Button>
		{/if}
	{:else if crmViewCurrency.failed}
		<span role="status" class="text-xs text-destructive">{crmViewCurrency.failed}: {text.viewCurrencyFailed}</span>
		<Button variant="outline" size="sm" onclick={() => handleChange(crmViewCurrency.failed)}>{text.retry}</Button>
	{/if}
	</div>
{/if}

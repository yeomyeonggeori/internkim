<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { loadConvertedAmount, type ConvertedAmount } from '$lib/currency/converted-amount';
	import { formatMoney } from './crm-money';
	import { majorAmountOf, type CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import type { CRMText } from './text';

	type Props = {
		amountMinor: number | null;
		currency: string;
		baseCurrency: string;
		settledAmountMinor: number | null;
		settledCurrency: string;
		currencyCatalogue: CurrencyCatalogue;
		text: CRMText;
	};

	let { amountMinor, currency, baseCurrency, settledAmountMinor, settledCurrency, currencyCatalogue, text }: Props = $props();

	const debounceDelayMilliseconds = 400;

	let estimate = $state<ConvertedAmount | null>(null);

	let isSettled = $derived(settledAmountMinor !== null && settledCurrency !== '');
	let shouldEstimate = $derived(
		!isSettled && amountMinor !== null && amountMinor > 0 && baseCurrency !== '' && currency !== baseCurrency
	);

	$effect(() => {
		const requestedAmountMinor = amountMinor;
		const requestedCurrency = currency;
		const requestedBaseCurrency = baseCurrency;
		if (!shouldEstimate || requestedAmountMinor === null) {
			estimate = null;
			return;
		}
		let isCancelled = false;
		const timeoutID = setTimeout(async () => {
			const result = await loadConvertedAmount(requestedAmountMinor, requestedCurrency, requestedBaseCurrency);
			if (isCancelled) return;
			estimate = result;
		}, debounceDelayMilliseconds);
		return () => {
			isCancelled = true;
			clearTimeout(timeoutID);
		};
	});
</script>

{#if isSettled}
	<p class="text-sm text-muted-foreground">
		{formatMoney(majorAmountOf(settledAmountMinor ?? 0, settledCurrency, currencyCatalogue), settledCurrency, '-', currentLocale.value)} · {text.conversionSettled}
	</p>
{:else if shouldEstimate && estimate}
	<p class="text-sm text-muted-foreground">
		{formatMoney(majorAmountOf(estimate.amountMinor, estimate.currencyCode, currencyCatalogue), estimate.currencyCode, '-', currentLocale.value)} · {text.conversionEstimate.replace('{date}', estimate.asOf)}
	</p>
{/if}

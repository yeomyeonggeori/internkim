<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { loadConvertedAmount, type ConvertedAmount } from '$lib/currency/converted-amount';
	import type { Money } from '$lib/money';
	import { formatOptionalMoney } from './crm-money';
	import type { CRMText } from './text';

	type Props = {
		amount: Money | undefined;
		baseCurrency: string;
		settledAmount: Money | undefined;
		text: CRMText;
	};

	let { amount, baseCurrency, settledAmount, text }: Props = $props();

	const debounceDelayMilliseconds = 400;

	let estimate = $state<ConvertedAmount | null>(null);

	let shouldEstimate = $derived(
		settledAmount === undefined && amount !== undefined && amount.amount > 0 && baseCurrency !== '' && amount.currency !== baseCurrency
	);

	$effect(() => {
		const requestedAmount = amount;
		const requestedBaseCurrency = baseCurrency;
		if (!shouldEstimate || requestedAmount === undefined) {
			estimate = null;
			return;
		}
		let isCancelled = false;
		const timeoutID = setTimeout(async () => {
			const result = await loadConvertedAmount(requestedAmount, requestedBaseCurrency);
			if (isCancelled) return;
			estimate = result;
		}, debounceDelayMilliseconds);
		return () => {
			isCancelled = true;
			clearTimeout(timeoutID);
		};
	});
</script>

{#if settledAmount}
	<p class="text-sm text-muted-foreground">
		{formatOptionalMoney(settledAmount, '-', currentLocale.value)} · {text.conversionSettled}
	</p>
{:else if shouldEstimate && estimate}
	<p class="text-sm text-muted-foreground">
		{formatOptionalMoney(estimate.amount, '-', currentLocale.value)} · {text.conversionEstimate.replace('{date}', estimate.asOf)}
	</p>
{/if}

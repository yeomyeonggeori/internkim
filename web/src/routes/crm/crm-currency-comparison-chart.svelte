<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import * as Card from '$lib/components/ui/card';
	import { buildCRMCurrencyComparisonRows } from './crm-currency-comparison';
	import { formatMoney } from './crm-money';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import type { CRMMoneyTotals } from './crm-types';
	import type { CRMText } from './text';

	type Props = {
		expectedTotals: CRMMoneyTotals;
		wonTotals: CRMMoneyTotals;
		currencyCatalogue: CurrencyCatalogue;
		text: CRMText;
	};

	let { expectedTotals, wonTotals, currencyCatalogue, text }: Props = $props();
	let rows = $derived(buildCRMCurrencyComparisonRows(currencyCatalogue, expectedTotals, wonTotals));
</script>

<Card.Root class="h-full min-w-0" data-crm-currency-comparison data-crm-report-card="currency">
	<Card.Header>
		<Card.Title class="text-base">{text.currencyValueComparison}</Card.Title>
		<Card.Description>{text.currencyValueComparisonDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="max-h-[28rem] overflow-y-auto pb-8 [mask-image:linear-gradient(to_bottom,black_calc(100%-2rem),transparent)] grid gap-5">
		{#each rows as row (row.currency)}
			<div
				class="grid min-w-0 gap-2"
				data-crm-currency-row={row.currency}
				role="group"
				aria-label={`${row.currency} ${text.openValue} ${formatMoney(row.expectedValue, row.currency, currencyCatalogue, text.noValue, currentLocale.value)}, ${text.wonValue} ${formatMoney(row.wonValue, row.currency, currencyCatalogue, text.noValue, currentLocale.value)}`}
			>
				<p class="text-sm font-semibold">{row.currency}</p>
				<div class="grid gap-2">
					<div class="grid grid-cols-[4.75rem_minmax(0,1fr)_minmax(5rem,auto)] items-center gap-2 text-xs">
						<span class="text-muted-foreground">{text.openValue}</span>
						<div class="h-2.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
							<div
								class="h-full rounded-full bg-primary"
								data-crm-currency-bar="expected"
								style={`width: ${row.expectedPercent}%`}
							></div>
						</div>
						<span class="text-right font-medium tabular-nums">{formatMoney(row.expectedValue, row.currency, currencyCatalogue, text.noValue, currentLocale.value)}</span>
					</div>
					<div class="grid grid-cols-[4.75rem_minmax(0,1fr)_minmax(5rem,auto)] items-center gap-2 text-xs">
						<span class="text-muted-foreground">{text.wonValue}</span>
						<div class="h-2.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
							<div
								class="h-full rounded-full bg-emerald-500"
								data-crm-currency-bar="won"
								style={`width: ${row.wonPercent}%`}
							></div>
						</div>
						<span class="text-right font-medium tabular-nums">{formatMoney(row.wonValue, row.currency, currencyCatalogue, text.noValue, currentLocale.value)}</span>
					</div>
				</div>
			</div>
		{:else}
			<p class="py-8 text-center text-sm text-muted-foreground">{text.noReportData}</p>
		{/each}
	</Card.Content>
</Card.Root>

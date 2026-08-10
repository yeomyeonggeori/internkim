<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { buildCRMCurrencyComparisonRows } from './crm-currency-comparison';
	import { formatMoney } from './crm-money';
	import type { CRMMoneyTotals } from './crm-types';
	import type { CRMText } from './text';

	type Props = {
		expectedTotals: CRMMoneyTotals;
		wonTotals: CRMMoneyTotals;
		text: CRMText;
	};

	let { expectedTotals, wonTotals, text }: Props = $props();
	let rows = $derived(buildCRMCurrencyComparisonRows(expectedTotals, wonTotals));
</script>

<Card.Root class="h-full min-w-0" data-crm-currency-comparison data-crm-report-card="currency">
	<Card.Header>
		<Card.Title class="text-base">{text.currencyValueComparison}</Card.Title>
		<Card.Description>{text.currencyValueComparisonDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-5">
		{#each rows as row (row.currency)}
			<div
				class="grid min-w-0 gap-2"
				data-crm-currency-row={row.currency}
				role="group"
				aria-label={`${row.currency} ${text.openValue} ${formatMoney(row.expectedValue, row.currency, text.noValue)}, ${text.wonValue} ${formatMoney(row.wonValue, row.currency, text.noValue)}`}
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
						<span class="text-right font-medium tabular-nums">{formatMoney(row.expectedValue, row.currency, text.noValue)}</span>
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
						<span class="text-right font-medium tabular-nums">{formatMoney(row.wonValue, row.currency, text.noValue)}</span>
					</div>
				</div>
			</div>
		{:else}
			<p class="py-8 text-center text-sm text-muted-foreground">{text.noReportData}</p>
		{/each}
	</Card.Content>
</Card.Root>

<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Meter } from '$lib/components/ui/meter';
	import { companyCredits, type CompanyCredits } from '$lib/company/credits';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { billingText } from './billing-text';
	import { billingDay, filled } from './billing-format';

	const text = createPageText(billingText);

	let credits = $state<CompanyCredits | null>(null);

	const usedPercent = $derived(
		credits ? Math.min(100, Math.round((100 * credits.myCreditsUsed) / Math.max(credits.usageLimit, 1))) : 0
	);

	onMount(async () => {
		credits = await companyCredits().catch(() => null);
	});
</script>

{#if credits?.paidBy === 'subscription'}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.myUsage}</Card.Title>
			<Card.Description>{text.myUsageDescription}</Card.Description>
		</Card.Header>
		<Card.Content class="grid gap-2">
			<div class="flex justify-end text-sm tabular-nums text-muted-foreground">
				{filled(text.myUsagePercent, { percent: String(usedPercent) })}
			</div>
			<Meter value={usedPercent} max={100} aria-label={text.myUsage} />
		</Card.Content>
		<Card.Footer class="flex-col items-start gap-0.5 text-sm text-muted-foreground">
			<span>{filled(text.myUsageResets, { date: billingDay(credits.periodEndsAt, currentLocale.value) })}</span>
			<span>{text.myUsageAtLimit}</span>
		</Card.Footer>
	</Card.Root>
{/if}

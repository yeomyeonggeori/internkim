<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Meter } from '$lib/components/ui/meter';
	import type { CompanyCredits } from '$lib/company/credits';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { billingText } from './billing-text';
	import { billingDay, creditCount, filled } from './billing-format';

	let { credits }: { credits: CompanyCredits } = $props();

	const text = createPageText(billingText);
	const periodGrant = $derived(Math.max((credits.billedSeats ?? 0) * credits.creditsPerSeat, credits.subscriptionCredits));
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.companyCredits}</Card.Title>
		<Card.Description>{text.companyCreditsDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-2">
		<div class="flex items-baseline justify-between gap-2 text-sm">
			<span class="font-medium">{text.subscriptionCredits}</span>
			<span class="text-muted-foreground tabular-nums">
				{filled(text.creditsLeft, { credits: creditCount(credits.subscriptionCredits, currentLocale.value) })}
			</span>
		</div>
		<Meter value={credits.subscriptionCredits} max={Math.max(periodGrant, 1)} aria-label={text.subscriptionCredits} />
		<p class="text-sm text-muted-foreground">
			{filled(text.creditsRefill, { date: billingDay(credits.periodEndsAt, currentLocale.value) })}
		</p>
	</Card.Content>
	<Card.Footer class="flex-col items-start gap-0.5 text-sm">
		<span class="font-medium">{filled(text.permanentCredits, { credits: creditCount(credits.permanentCredits, currentLocale.value) })}</span>
		<span class="text-muted-foreground">{text.permanentCreditsDescription}</span>
	</Card.Footer>
</Card.Root>

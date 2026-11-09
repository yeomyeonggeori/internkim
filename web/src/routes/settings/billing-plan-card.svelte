<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import type { CompanyCredits } from '$lib/company/credits';
	import { subscriptionCheckoutURL } from '$lib/company/credits';
	import { holdsAnOpenSubscription } from '$lib/company/credits-contract';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { toast } from 'svelte-sonner';
	import { billingText } from './billing-text';
	import { billingDay, creditCount, filled } from './billing-format';

	let { credits }: { credits: CompanyCredits } = $props();

	const text = createPageText(billingText);
	let isOpeningCheckout = $state(false);

	const perSeat = $derived(creditCount(credits.creditsPerSeat, currentLocale.value));

	async function subscribe() {
		isOpeningCheckout = true;
		try {
			window.location.assign(await subscriptionCheckoutURL());
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.subscribeFailed);
			isOpeningCheckout = false;
		}
	}
</script>

{#if credits.paidBy === 'subscription'}
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				{text.subscription}
				<Badge>{text.subscriptionActive}</Badge>
			</Card.Title>
			<Card.Description>
				{filled(text.subscriptionSeats, { seats: String(credits.billedSeats ?? 0), credits: perSeat })}
			</Card.Description>
		</Card.Header>
		<Card.Footer class="text-sm text-muted-foreground">
			{filled(text.subscriptionRenews, { date: billingDay(credits.periodEndsAt, currentLocale.value) })}
		</Card.Footer>
	</Card.Root>
{:else if holdsAnOpenSubscription(credits.subscriptionStatus)}
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				{text.subscription}
				<Badge variant="destructive">{text.subscriptionPastDue}</Badge>
			</Card.Title>
			<Card.Description>{text.subscriptionNeedsPayment}</Card.Description>
		</Card.Header>
	</Card.Root>
{:else if credits.paidBy === 'company_key'}
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				{text.companyKey}
				<Badge>{text.companyKeyInUse}</Badge>
			</Card.Title>
			<Card.Description>{text.companyKeyDescription}</Card.Description>
		</Card.Header>
		<Card.Footer class="flex flex-wrap items-center justify-between gap-3">
			<span class="text-sm text-muted-foreground">{text.companyKeyUsage}</span>
			<a href="/settings/setup" class={buttonVariants({ variant: 'outline' })}>{text.companyKeyChange}</a>
		</Card.Footer>
	</Card.Root>
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.subscription}</Card.Title>
			<Card.Description>{filled(text.subscriptionOffer, { credits: perSeat })}</Card.Description>
		</Card.Header>
		<Card.Footer class="justify-end">
			<Button variant="outline" onclick={subscribe} disabled={isOpeningCheckout}>{text.subscribe}</Button>
		</Card.Footer>
	</Card.Root>
{:else}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.subscription}</Card.Title>
			<Card.Description>{filled(text.subscriptionOffer, { credits: perSeat })}</Card.Description>
		</Card.Header>
		<Card.Footer class="flex flex-wrap items-center justify-between gap-3">
			<span class="text-sm text-muted-foreground">{isOpeningCheckout ? text.subscribing : ''}</span>
			<Button onclick={subscribe} disabled={isOpeningCheckout}>{text.subscribe}</Button>
		</Card.Footer>
	</Card.Root>
	<p class="text-sm text-muted-foreground">
		{text.companyKeyOffer}
		<a href="/settings/setup" class="underline">{text.companyKeyChange}</a>
	</p>
{/if}

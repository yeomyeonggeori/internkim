<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { companyCredits, type CompanyCredits } from '$lib/company/credits';
	import { holdsAnOpenSubscription, isSubscriptionPaying } from '$lib/company/credits-contract';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import BillingCreditsCard from './billing-credits-card.svelte';
	import BillingInvoicesCard from './billing-invoices-card.svelte';
	import BillingMemberUsageCard from './billing-member-usage-card.svelte';
	import BillingPlanCard from './billing-plan-card.svelte';
	import BillingTopUpCard from './billing-top-up-card.svelte';
	import { payerChangeWaitingOf } from './billing-payer-change';
	import { billingText } from './billing-text';

	const text = createPageText(billingText);

	let credits = $state<CompanyCredits | null>(null);
	let isPayerChangeWaiting = $state<boolean | null>(null);
	let isLoading = $state(true);
	let hasFailed = $state(false);

	async function load() {
		try {
			const loaded = await companyCredits();
			isPayerChangeWaiting = await payerChangeWaitingOf(loaded);
			credits = loaded;
			hasFailed = false;
		} catch {
			hasFailed = true;
		} finally {
			isLoading = false;
		}
	}

	onMount(load);
</script>

<header class="grid gap-1 pt-2 max-sm:pt-0">
	<h2 class="text-xl font-semibold max-sm:sr-only">{text.title}</h2>
	<p class="text-sm text-muted-foreground">{text.description}</p>
</header>
{#if isLoading}
	<div role="status" aria-label={text.title} aria-busy="true"><Skeleton class="h-32 w-full rounded-xl" /></div>
{:else if hasFailed}
	<p class="text-sm text-muted-foreground">{text.loadFailed}</p>
{:else if credits}
	<BillingPlanCard {credits} {isPayerChangeWaiting} />
	{#if credits.paidBy === 'subscription' || isSubscriptionPaying(credits.subscriptionStatus)}
		<BillingCreditsCard {credits} />
		<BillingTopUpCard />
		<BillingMemberUsageCard {credits} onchange={load} />
	{/if}
	<BillingInvoicesCard showsWhenEmpty={credits.paidBy === 'subscription' || holdsAnOpenSubscription(credits.subscriptionStatus)} />
{/if}

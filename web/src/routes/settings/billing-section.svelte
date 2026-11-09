<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { companyCredits, type CompanyCredits } from '$lib/company/credits';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import BillingCreditsCard from './billing-credits-card.svelte';
	import BillingMemberUsageCard from './billing-member-usage-card.svelte';
	import BillingPlanCard from './billing-plan-card.svelte';
	import BillingTopUpCard from './billing-top-up-card.svelte';
	import { billingText } from './billing-text';

	const text = createPageText(billingText);

	let credits = $state<CompanyCredits | null>(null);
	let isLoading = $state(true);
	let hasFailed = $state(false);

	async function load() {
		try {
			credits = await companyCredits();
			hasFailed = false;
		} catch {
			hasFailed = true;
		} finally {
			isLoading = false;
		}
	}

	onMount(load);
</script>

<header class="grid gap-1 pt-2">
	<h2 class="text-xl font-semibold">{text.title}</h2>
	<p class="text-sm text-muted-foreground">{text.description}</p>
</header>
{#if isLoading}
	<div role="status" aria-label={text.title} aria-busy="true"><Skeleton class="h-32 w-full rounded-xl" /></div>
{:else if hasFailed}
	<p class="text-sm text-muted-foreground">{text.loadFailed}</p>
{:else if credits}
	<BillingPlanCard {credits} />
	{#if credits.paidBy === 'subscription'}
		<BillingCreditsCard {credits} />
		<BillingTopUpCard />
		<BillingMemberUsageCard {credits} onchange={load} />
	{/if}
{/if}

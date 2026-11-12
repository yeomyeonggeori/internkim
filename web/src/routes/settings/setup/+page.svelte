<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { onMount } from 'svelte';
	import type { HostSetupStatus } from '$lib/company/host-setup';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CompanyMembers from '../company-members.svelte';
	import CompanyComputerCard from './company-computer-card.svelte';
	import CompanyComputerCardSkeleton from './company-computer-card-skeleton.svelte';
	import FirstMessage from './first-message.svelte';
	import { fetchHostSetupStatus } from './host-setup-client';
	import { stepHeading } from './step-heading';
	import { hostSetupText } from './text';

	const text = createPageText(hostSetupText);
	let status = $state<HostSetupStatus | null>(null);
	let isLoading = $state(true);
	let errorMessage = $state('');

	onMount(async () => {
		try {
			status = await fetchHostSetupStatus();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failed;
		} finally {
			isLoading = false;
		}
	});
</script>

<svelte:head><title>{text.title}</title></svelte:head>

<main class="min-h-0 flex-1 overflow-y-auto">
	<div class="mx-auto grid max-w-3xl gap-6 px-4 py-4 sm:px-6 sm:py-8">
		<header class="grid gap-2">
			<h1 class="text-2xl font-semibold">{text.title}</h1>
			<p class="text-muted-foreground">{text.description}</p>
		</header>
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		{#if isLoading}
			<div role="status" aria-label={text.loading} aria-busy="true" class="grid gap-6">
				<p aria-hidden="true" class="flex items-center gap-1 text-sm"><Skeleton class="h-4 w-24" /> · {text.accountReady}</p>
				<CompanyComputerCardSkeleton step={1} />
			</div>
		{:else if status}
			<p class="text-sm">{status.company.name} · {text.accountReady}</p>
			<CompanyComputerCard bind:status bind:errorMessage step={1} />
			<section class="grid gap-3">
				<h2 class="text-xl font-semibold">{stepHeading(text.invite, 2)}</h2>
				<p class="text-sm text-muted-foreground">{text.inviteDescription}</p>
				<CompanyMembers />
			</section>
			<FirstMessage companySlug={status.company.slug} bind:errorMessage step={3} />
		{/if}
	</div>
</main>

<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import WebAuthGate from '$lib/components/web-auth-gate.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { answerConsent, consentRequestOf, hostOf, type ConsentRequest } from '$lib/connected-apps';
	import { consentText } from './consent-text';

	let { data } = $props();

	const text = createPageText(consentText);
	const authorizationID = $derived(page.url.searchParams.get('authorization_id') ?? '');

	let request = $state<ConsentRequest | null>(null);
	let isAnswering = $state(false);
	let failure = $state('');

	$effect(() => {
		if (!data.session?.authenticated || !authorizationID) return;
		consentRequestOf(authorizationID).then((read) => {
			if (read.kind === 'decided') location.assign(read.redirectURL);
			request = read;
		});
	});

	async function answer(isApproved: boolean) {
		isAnswering = true;
		failure = '';
		try {
			location.assign(await answerConsent(authorizationID, isApproved));
		} catch (refusal) {
			failure = refusal instanceof Error ? refusal.message : text.unreadable;
			isAnswering = false;
		}
	}
</script>

<svelte:head><title>{text.title}</title></svelte:head>

<main class="flex min-h-dvh flex-1 items-center justify-center p-6">
	<WebAuthGate session={data.session} returnPath={page.url.pathname + page.url.search}>
		<Card.Root class="mx-auto w-full max-w-sm">
			<Card.Header>
				<Card.Title class="text-2xl">{text.title}</Card.Title>
				{#if !authorizationID}
					<Card.Description>{text.missing}</Card.Description>
				{:else if request === null}
					<Card.Description>{text.reading}</Card.Description>
				{:else if request.kind === 'unreadable'}
					<Card.Description>{text.unreadable}</Card.Description>
				{:else if request.kind === 'decided'}
					<Card.Description>{text.redirecting}</Card.Description>
				{:else}
					<Card.Description>
						{text.asking
							.replace('{client}', request.details.client.name || hostOf(request.details.redirect_uri))
							.replace('{email}', request.details.user.email)}
					</Card.Description>
				{/if}
			</Card.Header>
			{#if request?.kind === 'asking'}
				<Card.Content class="space-y-4">
					<p class="text-sm">{text.grants}</p>
					<p class="text-sm text-muted-foreground">
						{text.returnsTo.replace('{host}', hostOf(request.details.redirect_uri))}
					</p>
					{#if failure}
						<p class="text-sm text-destructive">{failure}</p>
					{/if}
					<div class="flex gap-2">
						<Button class="flex-1" variant="outline" disabled={isAnswering} onclick={() => answer(false)}>
							{text.deny}
						</Button>
						<Button class="flex-1" disabled={isAnswering} onclick={() => answer(true)}>
							{text.approve}
						</Button>
					</div>
				</Card.Content>
			{/if}
		</Card.Root>
	</WebAuthGate>
</main>

<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import * as Item from '$lib/components/ui/item';
	import { subscriptionCheckoutURL } from '$lib/company/credits';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { hostSetupText } from './text';

	const text = createPageText(hostSetupText);
	const isReturningFromCheckout = page.url.searchParams.get('subscription') === 'started';
	let isOpeningCheckout = $state(false);
	let errorMessage = $state('');

	async function subscribe() {
		isOpeningCheckout = true;
		errorMessage = '';
		try {
			window.location.assign(await subscriptionCheckoutURL(page.url.pathname));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.subscribeFailed;
			isOpeningCheckout = false;
		}
	}
</script>

<Item.Root variant="outline">
	<Item.Content>
		<Item.Title>{text.subscribe}</Item.Title>
		<Item.Description>{isReturningFromCheckout ? text.confirmingSubscription : text.subscribeDescription}</Item.Description>
	</Item.Content>
	{#if !isReturningFromCheckout}
		<Item.Actions>
			<Button onclick={subscribe} disabled={isOpeningCheckout}>{isOpeningCheckout ? text.subscribing : text.subscribe}</Button>
		</Item.Actions>
	{/if}
</Item.Root>
{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
<p class="text-sm text-muted-foreground">{text.orUseOwnModelKey}</p>

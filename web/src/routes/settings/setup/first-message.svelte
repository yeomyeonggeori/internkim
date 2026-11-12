<script lang="ts">
	import { goto } from '$app/navigation';
	import { ensureAgentConversation } from '$lib/components/channel/channel-api';
	import { Button } from '$lib/components/ui/button';
	import { companyPathOf } from '$lib/company-path';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { stepHeading } from './step-heading';
	import { hostSetupText } from './text';

	let { companySlug, errorMessage = $bindable(), step }: { companySlug: string; errorMessage: string; step?: number } = $props();

	const text = createPageText(hostSetupText);
	let isOpeningMessenger = $state(false);

	async function openMessenger() {
		isOpeningMessenger = true;
		try {
			const channelID = await ensureAgentConversation();
			await goto(`${companyPathOf(companySlug, '/messenger')}?channel=${encodeURIComponent(channelID)}`);
		} catch {
			errorMessage = text.unavailable;
		} finally {
			isOpeningMessenger = false;
		}
	}
</script>

<section class="grid justify-items-start gap-3">
	<h2 class="text-xl font-semibold">{stepHeading(text.tryIt, step)}</h2>
	<p class="text-sm text-muted-foreground">{text.tryItDescription}</p>
	<blockquote class="border-l-2 pl-4 text-sm">{text.firstMessage}</blockquote>
	<Button onclick={openMessenger} disabled={isOpeningMessenger}>{text.openMessenger}</Button>
</section>

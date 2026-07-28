<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cloudflareLoginURLFor, mattermostLoginURLFor, type WebAuthSession } from '$lib/web-auth-session';
	import PowerIcon from '@lucide/svelte/icons/power';
	import type { Snippet } from 'svelte';

	let { children, session, returnPath }: { children?: Snippet; session: WebAuthSession | null; returnPath: string } = $props();

	const text = createPageText(appShellText);
	const mattermostLoginURL = $derived(session?.mattermostLoginURL || mattermostLoginURLFor(returnPath));
	const cloudflareLoginURL = $derived(session?.cloudflareLoginURL || cloudflareLoginURLFor(returnPath));
</script>

{#if session?.authenticated}
	{@render children?.()}
{:else}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<Card.Root class="w-full max-w-sm">
			<Card.Header>
				<Card.Title>{text.signInTitle}</Card.Title>
				<Card.Description>{text.signInDescription}</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-3">
				<Button href={mattermostLoginURL} class="w-full gap-2">
					<PowerIcon class="size-4" />
					<span>{text.continueWithMattermost}</span>
				</Button>
				<Button href={cloudflareLoginURL} variant="outline" class="w-full gap-2">
					<PowerIcon class="size-4" />
					<span>{text.continueWithCloudflare}</span>
				</Button>
				{#if session?.isUnavailable}
					<p class="text-xs text-muted-foreground">{text.webSessionUnavailable}</p>
				{/if}
			</Card.Content>
		</Card.Root>
	</div>
{/if}

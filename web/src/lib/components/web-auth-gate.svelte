<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PowerIcon from '@lucide/svelte/icons/power';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { Snippet } from 'svelte';

	type SessionResponse = {
		authenticated: boolean;
		email?: string;
		identityEmail?: string;
		notInvited?: boolean;
		cloudflareLoginURL?: string;
	};

	let { children, returnPath }: { children?: Snippet; returnPath: string } = $props();
	const text = createPageText(appShellText);
	let isLoading = $state(true);
	let isAuthenticated = $state(false);
	let notInvited = $state(false);
	let identityEmail = $state('');
	let cloudflareLoginURL = $state('');
	let loadErrorMessage = $state('');
	let lastReturnPath = '';

	$effect(() => {
		if (returnPath === lastReturnPath) return;
		lastReturnPath = returnPath;
		loadSession();
	});

	async function loadSession() {
		isLoading = true;
		loadErrorMessage = '';
		try {
			const response = await fetch(`/auth/session?return=${encodeURIComponent(returnPath)}`, { credentials: 'include' });
			if (!response.ok) {
				throw new Error(`session returned ${response.status}`);
			}
			const session = (await response.json()) as SessionResponse;
			isAuthenticated = session.authenticated;
			notInvited = session.notInvited ?? false;
			identityEmail = session.identityEmail ?? '';
			cloudflareLoginURL = session.cloudflareLoginURL || cloudflareLoginURLForReturnPath();
		} catch {
			isAuthenticated = false;
			cloudflareLoginURL = cloudflareLoginURLForReturnPath();
			loadErrorMessage = text.webSessionUnavailable;
		} finally {
			isLoading = false;
		}
	}

	function cloudflareLoginURLForReturnPath() {
		return `/auth/cloudflare/start?return=${encodeURIComponent(returnPath)}`;
	}

	async function logOut() {
		try {
			const response = await fetch(`/auth/logout?return=${encodeURIComponent(returnPath)}`, {
				method: 'POST',
				credentials: 'include'
			});
			const body = (await response.json()) as { redirectURL?: string };
			location.replace(body.redirectURL ?? '/');
		} catch {
			location.replace('/');
		}
	}
</script>

{#if isLoading}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<div class="flex items-center gap-2 text-sm text-muted-foreground">
			<RefreshCwIcon class="size-4 animate-spin" />
			<span>{text.checkingSession}</span>
		</div>
	</div>
{:else if isAuthenticated}
	{@render children?.()}
{:else if notInvited}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<Card.Root class="w-full max-w-sm">
			<Card.Header>
				<Card.Title>{text.notInvitedTitle}</Card.Title>
				<Card.Description>{text.notInvitedDescription.replace('{email}', identityEmail)}</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-3">
				<Button variant="outline" class="w-full gap-2" onclick={logOut}>
					<PowerIcon class="size-4" />
					<span>{text.signOutTryAnother}</span>
				</Button>
			</Card.Content>
		</Card.Root>
	</div>
{:else}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<Card.Root class="w-full max-w-sm">
			<Card.Header>
				<Card.Title>{text.signInTitle}</Card.Title>
				<Card.Description>{text.signInDescription}</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-3">
				<Button href={cloudflareLoginURL} class="w-full gap-2">
					<PowerIcon class="size-4" />
					<span>{text.continueWithCloudflare}</span>
				</Button>
				{#if loadErrorMessage}
					<p class="text-xs text-muted-foreground">{loadErrorMessage}</p>
				{/if}
			</Card.Content>
		</Card.Root>
	</div>
{/if}

<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import PowerIcon from '@lucide/svelte/icons/power';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { Snippet } from 'svelte';
	import { buzzKeyLogin } from '$lib/buzz-key-login';
	import { isPasskeySupported } from '$lib/buzz-passkey';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

	type SessionResponse = {
		authenticated: boolean;
		email?: string;
		identityEmail?: string;
		notInvited?: boolean;
	};

	let { children, returnPath }: { children?: Snippet; returnPath: string } = $props();
	const text = createPageText(appShellText);
	const passkeyAvailable = isPasskeySupported();
	let isLoading = $state(true);
	let isAuthenticated = $state(false);
	let notInvited = $state(false);
	let identityEmail = $state('');
	let email = $state('');
	let password = $state('');
	let busy = $state(false);
	let errorMessage = $state('');
	let lastReturnPath = '';

	$effect(() => {
		if (returnPath === lastReturnPath) return;
		lastReturnPath = returnPath;
		loadSession();
	});

	async function loadSession() {
		isLoading = true;
		try {
			const session = (await fetch(`/auth/session?return=${encodeURIComponent(returnPath)}`, {
				credentials: 'include'
			}).then((response) => response.json())) as SessionResponse;
			isAuthenticated = session.authenticated;
			notInvited = session.notInvited ?? false;
			identityEmail = session.identityEmail ?? '';
		} catch {
			isAuthenticated = false;
		} finally {
			isLoading = false;
		}
	}

	function signupURL() {
		return `/auth/cloudflare/start?return=${encodeURIComponent(returnPath)}`;
	}

	async function login(factor: Parameters<typeof buzzKeyLogin>[1]) {
		if (email.trim().length === 0) {
			errorMessage = text.emailRequired;
			return;
		}
		busy = true;
		errorMessage = '';
		try {
			buzzIdentity.secretHex = await buzzKeyLogin(email.trim().toLowerCase(), factor);
			password = '';
			isAuthenticated = true;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.webSessionUnavailable;
		} finally {
			busy = false;
		}
	}

	async function logOut() {
		try {
			const body = (await fetch(`/auth/logout?return=${encodeURIComponent(returnPath)}`, {
				method: 'POST',
				credentials: 'include'
			}).then((response) => response.json())) as { redirectURL?: string };
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
			<Card.Content>
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
			<Card.Content class="space-y-4">
				<div class="flex flex-col gap-2">
					<Label for="login-email">{text.emailLabel}</Label>
					<Input id="login-email" type="email" autocomplete="username" bind:value={email} disabled={busy} />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="login-password">{text.passwordLabel}</Label>
					<Input id="login-password" type="password" autocomplete="current-password" bind:value={password} disabled={busy} />
				</div>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
				<Button class="w-full" onclick={() => login({ kind: 'password', password })} disabled={busy || password.length === 0}>
					{text.signInWithPassword}
				</Button>
				{#if passkeyAvailable}
					<Button variant="outline" class="w-full gap-2" onclick={() => login({ kind: 'passkey' })} disabled={busy}>
						<FingerprintIcon class="size-4" />
						{text.signInWithPasskey}
					</Button>
				{/if}
				<div class="border-t pt-3 text-center text-xs text-muted-foreground">
					{text.firstTimePrompt}
					<a class="ml-1 underline" href={signupURL()} data-sveltekit-reload>{text.signUpWithCloudflare}</a>
				</div>
			</Card.Content>
		</Card.Root>
	</div>
{/if}

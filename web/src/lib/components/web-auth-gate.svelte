<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Field, FieldDescription, FieldGroup, FieldLabel, FieldSeparator } from '$lib/components/ui/field';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cloudflareLoginURLFor, mattermostLoginURLFor, type WebAuthSession } from '$lib/web-auth-session';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import PowerIcon from '@lucide/svelte/icons/power';
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { buzzPasskeyLogin, buzzPasswordLogin, mattermostPasswordLogin } from '$lib/buzz-key-login';
	import { createBuzzIdentityTransport, enrollKnownBuzzIdentity } from '$lib/buzz-identity-session';
	import { isPasskeySupported } from '$lib/buzz-passkey';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

	let { children, session, returnPath }: { children?: Snippet; session: WebAuthSession | null; returnPath: string } = $props();

	const text = createPageText(appShellText);
	const fieldId = $props.id();
	const passkeyAvailable = isPasskeySupported();
	const identityTransport = createBuzzIdentityTransport();
	let buzzEnabled = $state(false);
	let email = $state('');
	let password = $state('');
	let busy = $state(false);
	let errorMessage = $state('');

	const mattermostLoginURL = $derived(session?.mattermostLoginURL || mattermostLoginURLFor(returnPath));
	const cloudflareLoginURL = $derived(session?.cloudflareLoginURL || cloudflareLoginURLFor(returnPath));

	// The company device runs Buzz identity (email/password/passkey signed in the
	// browser); the PoC tenants still authenticate through Mattermost/Cloudflare
	// SSO. The relay-config endpoint is unauthenticated, so the login screen can
	// pick the right flow before anyone signs in.
	onMount(async () => {
		try {
			const response = await fetch('/agent/api/buzz-relay-config', { credentials: 'include' });
			if (response.ok) {
				const document = (await response.json()) as { relayURL?: string };
				buzzEnabled = Boolean(document.relayURL?.trim());
			}
		} catch {
			buzzEnabled = false;
		}
	});

	function signupURL() {
		return `/auth/verify/start?return=${encodeURIComponent(returnPath)}`;
	}

	async function runLogin(work: () => Promise<string>) {
		busy = true;
		errorMessage = '';
		try {
			buzzIdentity.secretHex = await work();
			location.reload();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.webSessionUnavailable;
			busy = false;
		}
	}

	function loginWithPassword() {
		if (email.trim().length === 0) {
			errorMessage = text.emailRequired;
			return;
		}
		const normalizedEmail = email.trim().toLowerCase();
		return runLogin(async () => {
			try {
				return await buzzPasswordLogin(normalizedEmail, password);
			} catch (buzzError) {
				const secretHex = await mattermostPasswordLogin(normalizedEmail, password);
				await enrollKnownBuzzIdentity(identityTransport, secretHex, {
					kind: 'password',
					password
				}).catch(() => {});
				return secretHex;
			}
		});
	}

	function loginWithPasskey() {
		return runLogin(buzzPasskeyLogin);
	}
</script>

{#if session?.authenticated}
	{@render children?.()}
{:else if buzzEnabled}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<Card.Root class="mx-auto w-full max-w-sm">
			<Card.Header>
				<Card.Title class="text-2xl">{text.signInTitle}</Card.Title>
				<Card.Description>{text.signInDescription}</Card.Description>
			</Card.Header>
			<Card.Content>
				<form onsubmit={(event) => { event.preventDefault(); loginWithPassword(); }}>
					<FieldGroup>
						{#if passkeyAvailable}
							<Field>
								<Button type="button" class="w-full gap-2" onclick={loginWithPasskey} disabled={busy}>
									<FingerprintIcon class="size-4" />
									{text.signInWithPasskey}
								</Button>
							</Field>
							<FieldSeparator>{text.orSeparator}</FieldSeparator>
						{/if}
						<Field>
							<FieldLabel for="login-email-{fieldId}">{text.emailLabel}</FieldLabel>
							<Input id="login-email-{fieldId}" type="email" autocomplete="username" bind:value={email} disabled={busy} />
						</Field>
						<Field>
							<FieldLabel for="login-password-{fieldId}">{text.passwordLabel}</FieldLabel>
							<Input id="login-password-{fieldId}" type="password" autocomplete="current-password" bind:value={password} disabled={busy} />
						</Field>
						<Field>
							{#if errorMessage}
								<p class="text-sm text-destructive">{errorMessage}</p>
							{/if}
							<Button type="submit" class="w-full" disabled={busy || password.length === 0}>
								{text.signInWithPassword}
							</Button>
							<FieldDescription class="text-center">
								{text.firstTimePrompt}
								<a class="underline" href={signupURL()} data-sveltekit-reload>{text.signUpWithCloudflare}</a>
							</FieldDescription>
						</Field>
					</FieldGroup>
				</form>
			</Card.Content>
		</Card.Root>
	</div>
{:else}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<Card.Root class="mx-auto w-full max-w-sm">
			<Card.Header>
				<Card.Title class="text-2xl">{text.signInTitle}</Card.Title>
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

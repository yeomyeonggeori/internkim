<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Field, FieldDescription, FieldGroup, FieldLabel, FieldSeparator } from '$lib/components/ui/field';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { withReturnPath } from '$lib/return-path';
	import { cloudflareLoginURLFor, type WebAuthSession } from '$lib/web-auth-session';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import PowerIcon from '@lucide/svelte/icons/power';
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { isPasskeySupported as isBuzzPasskeySupported } from '$lib/buzz-passkey';
	import { isPasskeySupported as isSupabasePasskeySupported } from '$lib/supabase-passkey';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';
	import { isSupabaseConfigured, signInWithSupabase } from '$lib/supabase-session';
	import { signInWithPasskey } from '$lib/supabase-passkey';

	let { children, session, returnPath }: { children?: Snippet; session: WebAuthSession | null; returnPath: string } = $props();

	const text = createPageText(appShellText);
	const fieldId = $props.id();
	let servesCompanies = $state(false);
	let buzzEnabled = $state<boolean | null>(null);
	const passkeyAvailable = $derived(servesCompanies ? isSupabasePasskeySupported() : isBuzzPasskeySupported());
	let email = $state('');
	let password = $state('');
	let busy = $state(false);
	let errorMessage = $state('');

	const cloudflareLoginURL = $derived(session?.cloudflareLoginURL || cloudflareLoginURLFor(returnPath));

	onMount(async () => {
		servesCompanies = isSupabaseConfigured();
		if (servesCompanies) {
			buzzEnabled = true;
			return;
		}
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

	const signupURL = $derived(withReturnPath('/auth/verify/start', returnPath));
	const claimURL = $derived(withReturnPath('/auth/claim', returnPath));
	const startCompanyURL = $derived(withReturnPath('/auth/claim?new-company=1', returnPath));

	async function runLogin(work: () => Promise<string | null>) {
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
		if (servesCompanies) {
			return runLogin(async () => {
				await signInWithSupabase(normalizedEmail, password);
				return null;
			});
		}
		return runLogin(async () => {
			const { buzzPasswordLogin } = await import('$lib/buzz-key-login');
			return buzzPasswordLogin(normalizedEmail, password);
		});
	}

	function loginWithPasskey() {
		if (servesCompanies) {
			return runLogin(async () => {
				await signInWithPasskey();
				return null;
			});
		}
		return runLogin(async () => {
			const { buzzPasskeyLogin } = await import('$lib/buzz-key-login');
			return buzzPasskeyLogin();
		});
	}
</script>

{#if session?.authenticated}
	{@render children?.()}
{:else if buzzEnabled === null}
	<div class="flex min-h-0 flex-1 items-center justify-center p-6">
		<p class="text-sm text-muted-foreground">{text.checkingSession}</p>
	</div>
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
							{#if servesCompanies}
								<FieldDescription class="text-center">
									{text.firstTimePrompt}
									<a class="underline" href={claimURL}>{text.claimAccount}</a>
									<span class="px-1">·</span>
									<a class="underline" href={startCompanyURL}>{text.startCompany}</a>
								</FieldDescription>
							{:else}
								<FieldDescription class="text-center">
									{text.firstTimePrompt}
									<a class="underline" href={signupURL} data-sveltekit-reload>{text.signUpWithCloudflare}</a>
								</FieldDescription>
							{/if}
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
				<Button href={cloudflareLoginURL} class="w-full gap-2">
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

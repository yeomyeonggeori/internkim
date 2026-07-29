<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Field, FieldDescription, FieldGroup, FieldLabel, FieldSeparator } from '$lib/components/ui/field';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import PowerIcon from '@lucide/svelte/icons/power';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { Snippet } from 'svelte';
	import { buzzPasskeyLogin, buzzPasswordLogin, mattermostPasswordLogin } from '$lib/buzz-key-login';
	import { createBuzzIdentityTransport, enrollKnownBuzzIdentity } from '$lib/buzz-identity-session';
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
	const fieldId = $props.id();
	const passkeyAvailable = isPasskeySupported();
	const identityTransport = createBuzzIdentityTransport();
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
{/if}

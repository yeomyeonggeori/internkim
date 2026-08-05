<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Field, FieldDescription, FieldGroup, FieldLabel } from '$lib/components/ui/field';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isPasskeySupported, registerPasskey } from '$lib/supabase-passkey';
	import { sendClaimLink, setSupabasePassword, verifyClaimCode } from '$lib/supabase-session';
	import { isSupabaseConfigured, supabase } from '$lib/supabase';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import { onMount } from 'svelte';

	const text = createPageText(appShellText);
	const fieldID = $props.id();

	let step = $state<'address' | 'sent' | 'password' | 'passkey'>('address');
	let email = $state('');
	let code = $state('');
	let password = $state('');
	let busy = $state(false);
	let errorMessage = $state('');

	const description = $derived(
		step === 'address'
			? text.claimAddressDescription
			: step === 'sent'
				? text.claimSentDescription.replace('{email}', email)
				: step === 'password'
					? text.claimPasswordDescription
					: text.claimPasskeyDescription
	);

	async function run(work: () => Promise<void>) {
		busy = true;
		errorMessage = '';
		try {
			await work();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.claimFailed;
		} finally {
			busy = false;
		}
	}

	const askForLink = () =>
		run(async () => {
			await sendClaimLink(email);
			step = 'sent';
		});

	const proveTheAddress = () =>
		run(async () => {
			await verifyClaimCode(email, code);
			step = 'password';
		});

	const keepThePassword = () =>
		run(async () => {
			await setSupabasePassword(password);
			if (isPasskeySupported()) {
				step = 'passkey';
				return;
			}
			await goto('/flow/');
		});

	const keepThePasskey = () =>
		run(async () => {
			await registerPasskey();
			await goto('/flow/');
		});

	onMount(async () => {
		if (!isSupabaseConfigured()) return;
		const { data } = await supabase().auth.getSession();
		if (!data.session) return;
		email = data.session.user.email ?? '';
		step = 'password';
	});
</script>

<svelte:head><title>{text.claimTitle}</title></svelte:head>

<main class="flex min-h-svh items-center justify-center p-6">
	<Card.Root class="w-full max-w-sm">
		<Card.Header>
			<Card.Title class="text-2xl">{text.claimTitle}</Card.Title>
			<Card.Description>{description}</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if !isSupabaseConfigured()}
				<p class="text-sm text-destructive">{text.claimFailed}</p>
			{:else if step === 'address'}
				<form onsubmit={(event) => { event.preventDefault(); askForLink(); }}>
					<FieldGroup>
						<Field>
							<FieldLabel for="claim-email-{fieldID}">{text.emailLabel}</FieldLabel>
							<Input id="claim-email-{fieldID}" type="email" autocomplete="username" bind:value={email} disabled={busy} />
							<FieldDescription>{text.claimAddressHint}</FieldDescription>
						</Field>
						{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
						<Button type="submit" class="w-full" disabled={busy || !email.includes('@')}>{text.claimSendLink}</Button>
					</FieldGroup>
				</form>
			{:else if step === 'sent'}
				<form onsubmit={(event) => { event.preventDefault(); proveTheAddress(); }}>
					<FieldGroup>
						<Field>
							<FieldLabel for="claim-code-{fieldID}">{text.claimCodeLabel}</FieldLabel>
							<Input id="claim-code-{fieldID}" inputmode="numeric" autocomplete="one-time-code" bind:value={code} disabled={busy} />
							<FieldDescription>{text.claimCodeHint}</FieldDescription>
						</Field>
						{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
						<Button type="submit" class="w-full" disabled={busy || code.trim().length === 0}>{text.claimVerify}</Button>
						<Button variant="ghost" class="w-full" onclick={askForLink} disabled={busy}>{text.claimResend}</Button>
					</FieldGroup>
				</form>
			{:else if step === 'password'}
				<form onsubmit={(event) => { event.preventDefault(); keepThePassword(); }}>
					<FieldGroup>
						<Field>
							<FieldLabel for="claim-password-{fieldID}">{text.claimNewPasswordLabel}</FieldLabel>
							<Input id="claim-password-{fieldID}" type="password" autocomplete="new-password" bind:value={password} disabled={busy} />
							<FieldDescription>{text.claimPasswordHint}</FieldDescription>
						</Field>
						{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
						<Button type="submit" class="w-full" disabled={busy || password.length < 8}>{text.claimSavePassword}</Button>
					</FieldGroup>
				</form>
			{:else}
				<FieldGroup>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
					<Button class="w-full gap-2" onclick={keepThePasskey} disabled={busy}>
						<FingerprintIcon class="size-4" />
						{text.registerPasskey}
					</Button>
					<Button variant="ghost" class="w-full" onclick={() => goto('/flow/')} disabled={busy}>{text.claimSkipPasskey}</Button>
				</FieldGroup>
			{/if}
		</Card.Content>
	</Card.Root>
</main>

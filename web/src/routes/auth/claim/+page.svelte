<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import * as InputOTP from '$lib/components/ui/input-otp';
	import { Field, FieldDescription, FieldGroup, FieldLabel } from '$lib/components/ui/field';
	import { homePath } from '$lib/home-path';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isPasskeySupported, refusalOf, registerPasskey } from '$lib/supabase-passkey';
	import { askToClaim, setSupabasePassword, signInWithSupabase, verifyClaimCode } from '$lib/supabase-session';
	import { isSupabaseConfigured, supabase } from '$lib/supabase';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import { onMount } from 'svelte';

	const text = createPageText(appShellText);
	const fieldID = $props.id();

	let servesCompanies = $state(true);
	let step = $state<'address' | 'sent' | 'issued' | 'password' | 'passkey'>('address');
	let email = $state('');
	let code = $state('');
	let password = $state('');
	let issuedPassword = $state('');
	let busy = $state(false);
	let errorMessage = $state('');

	const describedStep: Record<typeof step, string> = $derived({
		address: text.claimAddressDescription,
		sent: text.claimSentDescription.replace('{email}', email),
		issued: text.claimIssuedDescription,
		password: text.claimPasswordDescription,
		passkey: text.claimPasskeyDescription
	});
	const description = $derived(describedStep[step]);

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

	const refusalText: Record<string, string> = {
		alreadyClaimed: text.claimAlreadyClaimed,
		tooManyLately: text.claimTooManyLately,
		failed: text.claimFailed
	};

	const askToClaimTheAddress = () =>
		run(async () => {
			const outcome = await askToClaim(email);
			if (outcome.kind === 'sent') {
				step = 'sent';
				return;
			}
			if (outcome.kind === 'issued') {
				issuedPassword = outcome.password;
				step = 'issued';
				return;
			}
			errorMessage = refusalText[outcome.kind] ?? text.claimFailed;
		});

	const goInWithTheIssuedPassword = () =>
		run(async () => {
			await signInWithSupabase(email, issuedPassword);
			if (isPasskeySupported()) {
				step = 'passkey';
				return;
			}
			await goto(homePath);
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
			await goto(homePath);
		});

	const keepThePasskey = () =>
		run(async () => {
			try {
				await registerPasskey();
			} catch (error) {
				if (refusalOf(error) === 'cancelled') return;
				throw new Error(text.claimPasskeyFailed);
			}
			await goto(homePath);
		});

	onMount(async () => {
		servesCompanies = isSupabaseConfigured();
		if (!servesCompanies) return;
		const { data } = await supabase().auth.getSession();
		if (!data.session) return;
		email = data.session.user.email ?? '';
		step = 'password';
	});
</script>

<svelte:head><title>{text.claimTitle}</title></svelte:head>

<main class="flex min-h-0 flex-1 items-center justify-center overflow-y-auto p-6">
	<Card.Root class="mx-auto w-full max-w-sm">
		<Card.Header>
			<Card.Title class="text-2xl">{text.claimTitle}</Card.Title>
			<Card.Description>{description}</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if !servesCompanies}
				<p class="text-sm text-destructive">{text.claimFailed}</p>
			{:else if step === 'address'}
				<form onsubmit={(event) => { event.preventDefault(); askToClaimTheAddress(); }}>
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
							<InputOTP.Root
								id="claim-code-{fieldID}"
								maxlength={6}
								bind:value={code}
								disabled={busy}
								onComplete={proveTheAddress}
							>
								{#snippet children({ cells })}
									<InputOTP.Group>
										{#each cells.slice(0, 3) as cell (cell)}
											<InputOTP.Slot {cell} />
										{/each}
									</InputOTP.Group>
									<InputOTP.Separator />
									<InputOTP.Group>
										{#each cells.slice(3, 6) as cell (cell)}
											<InputOTP.Slot {cell} />
										{/each}
									</InputOTP.Group>
								{/snippet}
							</InputOTP.Root>
							<FieldDescription>{text.claimCodeHint}</FieldDescription>
						</Field>
						{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
						<Button type="submit" class="w-full" disabled={busy || code.trim().length < 6}>{text.claimVerify}</Button>
						<Button variant="ghost" class="w-full" onclick={askToClaimTheAddress} disabled={busy}>{text.claimResend}</Button>
					</FieldGroup>
				</form>
			{:else if step === 'issued'}
				<FieldGroup>
					<Field>
						<FieldLabel for="claim-issued-{fieldID}">{text.claimNewPasswordLabel}</FieldLabel>
						<Input id="claim-issued-{fieldID}" readonly value={issuedPassword} class="font-mono" />
						<FieldDescription>{text.claimIssuedHint}</FieldDescription>
					</Field>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
					<Button class="w-full" onclick={goInWithTheIssuedPassword} disabled={busy}>{text.claimIssuedContinue}</Button>
				</FieldGroup>
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
					<Button variant="ghost" class="w-full" onclick={() => goto(homePath)} disabled={busy}>{text.claimSkipPasskey}</Button>
				</FieldGroup>
			{/if}
		</Card.Content>
	</Card.Root>
</main>

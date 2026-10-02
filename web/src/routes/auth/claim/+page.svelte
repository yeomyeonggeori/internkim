<script lang="ts">
	import { REGEXP_ONLY_DIGITS } from 'bits-ui';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import * as InputOTP from '$lib/components/ui/input-otp';
	import { Field, FieldDescription, FieldGroup, FieldLabel } from '$lib/components/ui/field';
	import { homePath } from '$lib/home-path';
	import { returnPathOf } from '$lib/return-path';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isPasskeySupported, refusalOf, registerPasskey } from '$lib/supabase-passkey';
	import { askToClaim, askToStartCompany, setSupabasePassword, verifyClaimCode } from '$lib/supabase-session';
	import { isSupabaseConfigured, supabase } from '$lib/supabase';
	import { claimCodeLength } from './claim-code';
	import { hasThePasswordStepExpired } from './password-step';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import { onMount } from 'svelte';

	const text = createPageText(appShellText);
	const fieldID = $props.id();

	let servesCompanies = $state(true);
	let step = $state<'address' | 'signedIn' | 'sent' | 'password' | 'passkey'>('address');
	let email = $state('');
	let code = $state('');
	let password = $state('');
	let busy = $state(false);
	let errorMessage = $state('');
	let passwordStepOpenedAt = 0;
	let hasChosenAnAddress = false;
	const isStartingCompany = $derived(page.url.searchParams.get('new-company') === '1');
	const whereToGoNext = $derived(returnPathOf(page.url) || homePath);

	const describedStep: Record<typeof step, string> = $derived({
		address: isStartingCompany ? text.startCompanyAddressDescription : text.claimAddressDescription,
		signedIn: isStartingCompany ? text.startCompanySignedInDescription : text.claimSignedInDescription,
		sent: (isStartingCompany ? text.startCompanySentDescription : text.claimSentDescription).replace('{email}', email),
		password: isStartingCompany ? text.startCompanyPasswordDescription : text.claimPasswordDescription,
		passkey: isStartingCompany ? text.startCompanyPasskeyDescription : text.claimPasskeyDescription
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
		tooManyLately: text.claimTooManyLately,
		failed: text.claimFailed
	};

	const askToClaimTheAddress = () =>
		run(async () => {
			code = '';
			hasChosenAnAddress = true;
			const outcome = await askToClaim(email);
			if (outcome.kind === 'sent') {
				step = 'sent';
				return;
			}
			errorMessage = refusalText[outcome.kind] ?? text.claimFailed;
		});

	const askToStartTheCompany = () =>
		run(async () => {
			hasChosenAnAddress = true;
			await askToStartCompany(email);
			step = 'sent';
		});

	function openThePasswordStep() {
		passwordStepOpenedAt = Date.now();
		code = '';
		password = '';
		step = 'password';
	}

	function startOver() {
		errorMessage = '';
		code = '';
		step = 'address';
	}

	const proveTheAddress = () =>
		run(async () => {
			await verifyClaimCode(email, code);
			openThePasswordStep();
		});

	const keepThePassword = () =>
		run(async () => {
			if (hasThePasswordStepExpired(passwordStepOpenedAt, Date.now())) {
				step = 'signedIn';
				throw new Error(text.claimExpired);
			}
			await setSupabasePassword(password);
			if (isPasskeySupported()) {
				step = 'passkey';
				return;
			}
			await goto(whereToGoNext);
		});

	const keepThePasskey = () =>
		run(async () => {
			try {
				await registerPasskey();
			} catch (error) {
				if (refusalOf(error) === 'cancelled') return;
				throw new Error(text.claimPasskeyFailed);
			}
			await goto(whereToGoNext);
		});

	onMount(async () => {
		servesCompanies = isSupabaseConfigured();
		if (!servesCompanies) return;
		const { data } = await supabase().auth.getSession();
		if (!data.session || hasChosenAnAddress) return;
		email = data.session.user.email ?? '';
		step = 'signedIn';
	});
</script>

<svelte:head><title>{isStartingCompany ? text.startCompanyTitle : text.claimTitle}</title></svelte:head>

<main class="flex min-h-svh items-center justify-center overflow-y-auto p-4 sm:p-6">
	<Card.Root class="mx-auto w-full max-w-sm">
		<Card.Header>
			<Card.Title class="text-2xl">{isStartingCompany ? text.startCompanyTitle : text.claimTitle}</Card.Title>
			<Card.Description>{description}</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if !servesCompanies}
				<p class="text-sm text-destructive">{text.claimFailed}</p>
			{:else if step === 'address'}
				<form onsubmit={(event) => { event.preventDefault(); isStartingCompany ? askToStartTheCompany() : askToClaimTheAddress(); }}>
					<FieldGroup>
						<Field>
							<FieldLabel for="claim-email-{fieldID}">{text.emailLabel}</FieldLabel>
							<Input id="claim-email-{fieldID}" type="email" autocomplete="username" bind:value={email} disabled={busy} />
							<FieldDescription>{isStartingCompany ? text.startCompanyEmailHint : text.claimAddressHint}</FieldDescription>
						</Field>
						{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
						<Button type="submit" class="w-full" disabled={busy || !email.includes('@')}>{isStartingCompany ? text.startCompanySendLink : text.claimSendLink}</Button>
					</FieldGroup>
				</form>
			{:else if step === 'signedIn'}
				<FieldGroup>
					<FieldDescription>{text.claimSignedInHint}</FieldDescription>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
					<Button type="button" class="w-full" onclick={isStartingCompany ? askToStartTheCompany : askToClaimTheAddress} disabled={busy}>
						{isStartingCompany ? text.startCompanySendCode : text.claimSendCode}
					</Button>
					<Button variant="ghost" class="w-full" onclick={startOver} disabled={busy}>{text.claimUseAnotherAddress}</Button>
				</FieldGroup>
			{:else if step === 'sent'}
				<form onsubmit={(event) => { event.preventDefault(); proveTheAddress(); }}>
					<FieldGroup>
						<Field>
							<FieldLabel for="claim-code-{fieldID}">{text.claimCodeLabel}</FieldLabel>
							<InputOTP.Root
								class="w-full gap-1.5"
								inputId="claim-code-{fieldID}"
								pushPasswordManagerStrategy="none"
								pattern={REGEXP_ONLY_DIGITS}
								maxlength={claimCodeLength}
								bind:value={code}
								disabled={busy}
								onComplete={proveTheAddress}
							>
								{#snippet children({ cells })}
									<InputOTP.Group class="min-w-0 flex-1">
										{#each cells.slice(0, Math.ceil(claimCodeLength / 2)) as cell (cell)}
											<InputOTP.Slot {cell} class="h-11 min-w-0 flex-1" />
										{/each}
									</InputOTP.Group>
									<InputOTP.Separator />
									<InputOTP.Group class="min-w-0 flex-1">
										{#each cells.slice(Math.ceil(claimCodeLength / 2)) as cell (cell)}
											<InputOTP.Slot {cell} class="h-11 min-w-0 flex-1" />
										{/each}
									</InputOTP.Group>
								{/snippet}
							</InputOTP.Root>
							<FieldDescription>{text.claimCodeHint}</FieldDescription>
						</Field>
						{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
						<Button type="submit" class="w-full" disabled={busy || code.trim().length < claimCodeLength}>{text.claimVerify}</Button>
						<Button variant="ghost" class="w-full" onclick={isStartingCompany ? askToStartTheCompany : askToClaimTheAddress} disabled={busy}>{text.claimResend}</Button>
						<Button variant="ghost" class="w-full" onclick={startOver} disabled={busy}>{text.claimUseAnotherAddress}</Button>
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
					<Button variant="ghost" class="w-full" onclick={() => goto(whereToGoNext)} disabled={busy}>{text.claimSkipPasskey}</Button>
				</FieldGroup>
			{/if}
		</Card.Content>
	</Card.Root>
</main>

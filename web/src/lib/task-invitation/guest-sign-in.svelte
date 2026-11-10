<script lang="ts">
	import { onMount } from 'svelte';
	import { REGEXP_ONLY_DIGITS } from 'bits-ui';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as InputOTP from '$lib/components/ui/input-otp';
	import * as Field from '$lib/components/ui/field';
	import { emailCodeLength } from '$lib/auth/email-code';
	import { isPasskeySupported, refusalOf, signInWithPasskey } from '$lib/supabase-passkey';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { sharedTasksText } from './text';
	import { requestGuestCode, verifyGuestCode } from './guest';

	let { onSignedIn }: { onSignedIn: () => Promise<void> } = $props();
	const text = createPageText(sharedTasksText);
	const fieldID = $props.id();
	let email = $state('');
	let code = $state('');
	let isCodeSent = $state(false);
	let isEnteringCode = $state(false);
	const asksForCode = $derived(isCodeSent || isEnteringCode);
	let isBusy = $state(false);
	let failure = $state('');
	let canUsePasskey = $state(false);

	onMount(() => {
		canUsePasskey = isPasskeySupported();
	});

	async function attempt(action: () => Promise<void>) {
		isBusy = true;
		failure = '';
		try {
			await action();
		} catch (error) {
			failure = error instanceof Error ? error.message : String(error);
		} finally {
			isBusy = false;
		}
	}

	function submit(event: SubmitEvent) {
		event.preventDefault();
		void attempt(async () => {
			if (!asksForCode) {
				await requestGuestCode(email.trim());
				isCodeSent = true;
				return;
			}
			await verifyGuestCode(email.trim(), code.trim());
			await onSignedIn();
		});
	}

	function signInByPasskey() {
		void attempt(async () => {
			try {
				await signInWithPasskey();
			} catch (error) {
				if (refusalOf(error) === 'cancelled') return;
				throw error;
			}
			await onSignedIn();
		});
	}
</script>

<form onsubmit={submit} class="grid gap-4">
	<Field.Field>
		<Field.Label for="{fieldID}-email">{text.email}</Field.Label>
		<Input id="{fieldID}-email" type="email" required autocomplete="email" bind:value={email} disabled={isCodeSent || isBusy} />
		{#if isCodeSent}<Field.Description>{text.codeSent.replace('{email}', email.trim())}</Field.Description>{/if}
	</Field.Field>
	{#if asksForCode}
		<Field.Field>
			<Field.Label for="{fieldID}-code">{text.code}</Field.Label>
			<InputOTP.Root inputId="{fieldID}-code" pushPasswordManagerStrategy="none" maxlength={emailCodeLength} required
				bind:value={code} disabled={isBusy} pattern={REGEXP_ONLY_DIGITS}>
				{#snippet children({ cells })}
					<InputOTP.Group>
						{#each cells.slice(0, emailCodeLength / 2) as cell (cell)}<InputOTP.Slot {cell} />{/each}
					</InputOTP.Group>
					<InputOTP.Separator />
					<InputOTP.Group>
						{#each cells.slice(emailCodeLength / 2) as cell (cell)}<InputOTP.Slot {cell} />{/each}
					</InputOTP.Group>
				{/snippet}
			</InputOTP.Root>
		</Field.Field>
	{/if}
	{#if failure}<p role="alert" class="text-sm text-destructive">{failure}</p>{/if}
	<div class="flex flex-wrap gap-2">
		<Button type="submit" disabled={isBusy || (asksForCode && code.length !== emailCodeLength)}>
			{asksForCode ? text.signIn : text.sendCode}
		</Button>
		{#if !asksForCode}
			<Button type="button" variant="link" disabled={isBusy} onclick={() => (isEnteringCode = true)}>{text.haveCode}</Button>
		{/if}
		{#if canUsePasskey && !asksForCode}
			<Button type="button" variant="outline" disabled={isBusy} onclick={signInByPasskey}>{text.signInWithPasskey}</Button>
		{/if}
	</div>
</form>

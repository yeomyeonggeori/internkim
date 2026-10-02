<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as InputOTP from '$lib/components/ui/input-otp';
	import * as Field from '$lib/components/ui/field';
	import { emailCodeLength } from '$lib/auth/email-code';
	import { supabase } from '$lib/supabase';
	import { dataRoomRequest } from '$lib/data-room/guest';

	let email = $state('');
	let code = $state('');
	let isCodeSent = $state(false);
	let isBusy = $state(false);
	let failure = $state('');

	async function accept() {
		const answer = await dataRoomRequest(`/api/v1/data-room/invitations/${page.params.shareID}`, 'POST');
		await goto(`/share/${z.object({ companyID: z.string().uuid() }).parse(answer).companyID}`);
	}

	async function submit() {
		isBusy = true;
		failure = '';
		try {
			if (!isCodeSent) {
				const { error } = await supabase().auth.signInWithOtp({ email: email.trim() });
				if (error) throw new Error(error.message);
				isCodeSent = true;
			} else {
				const { error } = await supabase().auth.verifyOtp({ email: email.trim(), token: code.trim(), type: 'email' });
				if (error) throw new Error(error.message);
				await accept();
			}
		} catch (error) { failure = error instanceof Error ? error.message : String(error); }
		finally { isBusy = false; }
	}

	onMount(async () => {
		const { data } = await supabase().auth.getSession();
		if (!data.session) return;
		email = data.session.user.email ?? '';
		try { await accept(); }
		catch (error) { failure = error instanceof Error ? error.message : String(error); }
	});
</script>

<svelte:head><title>Data room invitation</title></svelte:head>
<main class="mx-auto max-w-md px-6 py-16">
	<h1 class="text-2xl font-semibold">Data room invitation</h1>
	<p class="mt-3 text-sm text-muted-foreground">Sign in with the email that received this invitation.</p>
	<form onsubmit={(event) => { event.preventDefault(); void submit(); }} class="mt-6 space-y-4">
		<div><label for="guest-email" class="mb-2 block text-sm">Email</label><Input id="guest-email" type="email" required bind:value={email} disabled={isCodeSent} autocomplete="email" /></div>
		{#if isCodeSent}
			<Field.Field>
				<Field.Label for="guest-code">Email verification code</Field.Label>
				<InputOTP.Root inputId="guest-code" maxlength={emailCodeLength} required bind:value={code} disabled={isBusy} pattern="[0-9]*">
					{#snippet children({ cells })}
						<InputOTP.Group>
							{#each cells.slice(0, emailCodeLength / 2) as cell (cell)}
								<InputOTP.Slot {cell} />
							{/each}
						</InputOTP.Group>
						<InputOTP.Separator />
						<InputOTP.Group>
							{#each cells.slice(emailCodeLength / 2) as cell (cell)}
								<InputOTP.Slot {cell} />
							{/each}
						</InputOTP.Group>
					{/snippet}
				</InputOTP.Root>
			</Field.Field>
		{/if}
		{#if failure}<p role="alert" class="text-sm text-destructive">{failure}</p>{/if}
		<Button type="submit" disabled={isBusy || (isCodeSent && code.length !== emailCodeLength)}>{isCodeSent ? 'Open data room' : 'Send verification code'}</Button>
		{#if isCodeSent}<Button variant="ghost" onclick={() => { isCodeSent = false; code = ''; }}>Use another email</Button>{/if}
	</form>
</main>

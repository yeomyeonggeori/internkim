<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Field, FieldDescription, FieldGroup, FieldLabel } from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { slugShape } from '$lib/company-path';
	import { belongsToACompany, checkCompanyAddress, foundCompany } from '$lib/company/found-company';
	import { homePath } from '$lib/home-path';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { returnPathOf } from '$lib/return-path';
	import { isSupabaseConfigured } from '$lib/supabase';
	import { signOutOfSupabase } from '$lib/supabase-session';
	import { onMount } from 'svelte';
	import { startCompanyText } from './text';

	type AddressState = 'empty' | 'checking' | 'usable' | 'taken' | 'reserved' | 'shape' | 'needsLatin' | 'unchecked';

	const text = createPageText(startCompanyText);
	const fieldID = $props.id();
	const addressZone = $derived(page.data.addressZone);
	const signedInAs = $derived(page.data.session?.email ?? '');
	const whereTheyWereGoing = $derived(returnPathOf(page.url));
	const addressCheckDelayMilliseconds = 300;

	let name = $state('');
	let slug = $state('');
	let founderName = $state('');
	let hasTypedAddress = $state(false);
	let addressState = $state<AddressState>('empty');
	let isFounding = $state(false);
	let errorMessage = $state('');
	let isSigningOut = $state(false);
	let addressCheckTimer: ReturnType<typeof setTimeout> | undefined;

	const addressNotice = $derived(
		{
			empty: '',
			checking: text.addressChecking,
			usable: text.addressUsable,
			taken: text.addressTaken,
			reserved: text.addressReserved,
			shape: text.addressShape,
			needsLatin: text.addressNeedsLatin,
			unchecked: text.addressUnchecked
		}[addressState]
	);
	const canCreate = $derived(!isFounding && name.trim() !== '' && founderName.trim() !== '' && addressState === 'usable');

	function addressFromName(companyName: string): string {
		return companyName
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.slice(0, 40)
			.replace(/^-+|-+$/g, '');
	}

	function followTheName() {
		if (hasTypedAddress) return;
		slug = addressFromName(name);
		if (slug === '') {
			clearTimeout(addressCheckTimer);
			addressState = name.trim() === '' ? 'empty' : 'needsLatin';
			return;
		}
		scheduleAddressCheck();
	}

	function typeTheAddress() {
		hasTypedAddress = slug !== '';
		scheduleAddressCheck();
	}

	function scheduleAddressCheck() {
		clearTimeout(addressCheckTimer);
		const asked = slug.trim().toLowerCase();
		if (asked === '') {
			addressState = 'empty';
			return;
		}
		if (!slugShape.test(asked)) {
			addressState = 'shape';
			return;
		}
		addressState = 'checking';
		addressCheckTimer = setTimeout(() => void checkAddress(asked), addressCheckDelayMilliseconds);
	}

	async function checkAddress(asked: string) {
		try {
			const answer = await checkCompanyAddress(asked);
			if (asked !== slug.trim().toLowerCase()) return;
			addressState = answer.usable ? 'usable' : answer.taken ? 'taken' : 'reserved';
		} catch {
			if (asked === slug.trim().toLowerCase()) addressState = 'unchecked';
		}
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		isFounding = true;
		errorMessage = '';
		try {
			await foundCompany({
				name: name.trim(),
				slug: slug.trim().toLowerCase(),
				founderName: founderName.trim(),
				locale: currentLocale.value,
				timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
			});
			await goto(whereTheyWereGoing && whereTheyWereGoing !== homePath ? whereTheyWereGoing : '/settings/setup');
		} catch {
			errorMessage = text.createFailed;
			scheduleAddressCheck();
		} finally {
			isFounding = false;
		}
	}

	async function signOut() {
		isSigningOut = true;
		await signOutOfSupabase();
		location.replace(homePath);
	}

	onMount(async () => {
		if (!isSupabaseConfigured()) return;
		if (await belongsToACompany()) await goto(whereTheyWereGoing || homePath);
	});
</script>

<svelte:head><title>{text.title}</title></svelte:head>

<main class="flex min-h-svh items-center justify-center overflow-y-auto p-4 sm:p-6">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<Card.Title class="text-2xl">{text.title}</Card.Title>
			{#if signedInAs}
				<Card.Description>{text.signedInAs.replace('{email}', signedInAs)}</Card.Description>
			{/if}
		</Card.Header>
		<Card.Content>
			<form onsubmit={create}>
				<FieldGroup>
					<Field>
						<FieldLabel for="company-name-{fieldID}">{text.companyName}</FieldLabel>
						<Input id="company-name-{fieldID}" autocomplete="organization" bind:value={name} oninput={followTheName} disabled={isFounding} />
					</Field>
					<Field>
						<FieldLabel for="company-address-{fieldID}">{text.companyAddress}</FieldLabel>
						<InputGroup.Root>
							<InputGroup.Addon>
								<InputGroup.Text>{addressZone}/</InputGroup.Text>
							</InputGroup.Addon>
							<InputGroup.Input
								id="company-address-{fieldID}"
								class="pl-0.5"
								autocapitalize="off"
								autocomplete="off"
								spellcheck={false}
								bind:value={slug}
								oninput={typeTheAddress}
								disabled={isFounding}
							/>
						</InputGroup.Root>
						{#if addressNotice}
							<FieldDescription class={['usable', 'checking', 'needsLatin'].includes(addressState) ? '' : 'text-destructive'}>{addressNotice}</FieldDescription>
						{/if}
						<FieldDescription>{text.companyAddressHint}</FieldDescription>
					</Field>
					<Field>
						<FieldLabel for="founder-name-{fieldID}">{text.founderName}</FieldLabel>
						<Input id="founder-name-{fieldID}" autocomplete="name" bind:value={founderName} disabled={isFounding} />
						<FieldDescription>{text.founderNameHint}</FieldDescription>
					</Field>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
					<Button type="submit" class="w-full" disabled={!canCreate}>{isFounding ? text.creating : text.create}</Button>
				</FieldGroup>
			</form>
		</Card.Content>
		{#if isSupabaseConfigured()}
			<Card.Footer class="justify-center">
				<Button variant="link" size="sm" class="text-muted-foreground" onclick={signOut} disabled={isSigningOut}>{text.signInAsAnother}</Button>
			</Card.Footer>
		{/if}
	</Card.Root>
</main>

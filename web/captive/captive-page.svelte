<script lang="ts">
	import { onMount } from 'svelte';
	import Lock from '@lucide/svelte/icons/lock';
	import Plus from '@lucide/svelte/icons/plus';
	import Wifi from '@lucide/svelte/icons/wifi';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { Label } from '$lib/components/ui/label';
	import { Spinner } from '$lib/components/ui/spinner';
	import logoURL from '../static/logo.svg';
	import BackgroundPattern from './background-pattern.svelte';
	import { fetchNetworkListing, submitJoin, type JoinOutcome, type ListedNetwork } from './captive-api';
	import { captiveText, preferredLanguage } from './captive-text';

	const manualChoice = '';
	const language = preferredLanguage(navigator.languages);
	const text = captiveText[language];

	let networks = $state<ListedNetwork[]>([]);
	let isScanning = $state(true);
	let hasJoinFailed = $state(false);
	let chosenSSID = $state(manualChoice);
	let customSSID = $state('');
	let password = $state('');
	let isSubmitting = $state(false);
	let isJoining = $state(false);
	let refusal = $state<Exclude<JoinOutcome, 'accepted'> | undefined>(undefined);

	const isManual = $derived(chosenSSID === manualChoice);

	onMount(() => {
		document.documentElement.lang = language;
		document.title = text.setupLabel;
		const darkScheme = window.matchMedia('(prefers-color-scheme: dark)');
		document.documentElement.classList.toggle('dark', darkScheme.matches);
		fetchNetworkListing()
			.then((listing) => {
				networks = listing.networks;
				hasJoinFailed = listing.hasJoinFailed;
				chosenSSID = listing.networks[0]?.ssid ?? manualChoice;
			})
			.catch((errorValue: unknown) => {
				console.error('captive page could not load networks', { errorValue });
				refusal = 'unreachable';
			})
			.finally(() => {
				isScanning = false;
			});
	});

	async function connect(event: SubmitEvent) {
		event.preventDefault();
		isSubmitting = true;
		refusal = undefined;
		const outcome = await submitJoin(chosenSSID, isManual ? customSSID : '', password);
		isSubmitting = false;
		if (outcome === 'accepted') {
			isJoining = true;
			return;
		}
		refusal = outcome;
	}
</script>

<div class="relative flex min-h-dvh items-center justify-center bg-[var(--brand)] px-4 py-10 break-keep">
	<BackgroundPattern />
	<Card.Root class="relative w-full max-w-sm shadow-xl">
		<Card.Header class="flex flex-col items-center gap-2 text-center">
			<img src={logoURL} alt="" class="size-14 rounded-2xl" />
			<span class="text-muted-foreground text-xs font-semibold tracking-wide uppercase">{text.setupLabel}</span>
			<Card.Title class="text-lg text-balance">{isJoining ? text.joiningHeading : text.heading}</Card.Title>
		</Card.Header>
		<Card.Content>
			{#if isJoining}
				<div class="flex flex-col items-center gap-3 text-center">
					<Spinner class="size-6" />
					<p class="text-muted-foreground text-sm">{text.joiningInstruction}</p>
				</div>
			{:else}
				<form class="flex flex-col gap-4" onsubmit={connect}>
					{#if hasJoinFailed}
						<Alert.Root variant="destructive">
							<TriangleAlert />
							<Alert.Description>{text.joinFailure}</Alert.Description>
						</Alert.Root>
					{/if}
					<fieldset class="flex flex-col gap-2">
						<legend class="mb-2 text-sm font-medium">{text.networkLabel}</legend>
						{#if isScanning}
							<p class="text-muted-foreground flex items-center gap-2 text-sm"><Spinner />{text.scanning}</p>
						{:else}
							{#if networks.length === 0}
								<p class="text-muted-foreground text-sm">{text.noNetworks}</p>
							{/if}
							<Item.Group class="max-h-56 gap-1 overflow-y-auto overscroll-contain">
								{#each networks as network (network.ssid)}
									{@render networkChoice(network.ssid, network.ssid, network.isSecured)}
								{/each}
								{@render networkChoice(manualChoice, text.enterManually, false)}
							</Item.Group>
						{/if}
					</fieldset>
					{#if isManual}
						<div class="flex flex-col gap-2">
							<Label for="customSSID">{text.customNetworkLabel}</Label>
							<Input id="customSSID" bind:value={customSSID} autocomplete="off" />
						</div>
					{/if}
					<div class="flex flex-col gap-2">
						<Label for="password">{text.passwordLabel}</Label>
						<Input id="password" type="password" bind:value={password} autocomplete="off" />
					</div>
					{#if refusal}
						<p class="text-destructive text-sm" role="alert">{text[refusal]}</p>
					{/if}
					<Button type="submit" size="lg" disabled={isSubmitting}>
						{#if isSubmitting}<Spinner />{/if}
						{text.connect}
					</Button>
				</form>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

{#snippet networkChoice(value: string, label: string, isSecured: boolean)}
	<Item.Root variant={chosenSSID === value ? 'outline' : 'default'} size="sm">
		{#snippet child({ props })}
			<label {...props}>
				<input type="radio" name="ssid" {value} bind:group={chosenSSID} class="sr-only" />
				<Item.Media>
					{#if value === manualChoice}<Plus class="size-4" />{:else}<Wifi class="size-4" />{/if}
				</Item.Media>
				<Item.Content class="min-w-0">
					<Item.Title class="truncate">{label}</Item.Title>
				</Item.Content>
				{#if isSecured}<Lock class="text-muted-foreground size-4" />{/if}
			</label>
		{/snippet}
	</Item.Root>
{/snippet}

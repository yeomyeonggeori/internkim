<script lang="ts">
	import { page } from '$app/state';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import * as Select from '$lib/components/ui/select';
	import WebAuthGate from '$lib/components/web-auth-gate.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { allowConnection, answerConsent, consentRequestOf, type ConsentRequest } from '$lib/connected-apps';
	import {
		connectedAppPermissions,
		unchosenConnectedAppPermission,
		type ConnectedAppPermission
	} from '$lib/public-api-permission';
	import { hostOf, returnsToThisComputer } from '$lib/consent-return';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import { consentText } from './consent-text';

	let { data } = $props();

	const text = createPageText(consentText);
	const authorizationID = $derived(page.url.searchParams.get('authorization_id') ?? '');

	let request = $state<ConsentRequest | null>(null);
	let isAnswering = $state(false);
	let failure = $state('');
	let permission = $state<ConnectedAppPermission>(unchosenConnectedAppPermission);

	const permissionLabels = $derived<Record<ConnectedAppPermission, string>>({
		read: text.reads,
		write: text.writes
	});

	$effect(() => {
		if (!data.session?.authenticated || !authorizationID) return;
		consentRequestOf(authorizationID).then((read) => {
			if (read.kind === 'decided') location.assign(read.redirectURL);
			request = read;
		});
	});

	async function answer(isApproved: boolean) {
		if (request?.kind !== 'asking') return;
		isAnswering = true;
		failure = '';
		try {
			const clientID = request.details.client.id;
			location.assign(
				isApproved
					? await allowConnection(authorizationID, clientID, permission)
					: await answerConsent(authorizationID, false)
			);
		} catch (refusal) {
			failure = refusal instanceof Error ? refusal.message : text.unreadable;
			isAnswering = false;
		}
	}
</script>

<svelte:head><title>{text.title}</title></svelte:head>

<main class="flex min-h-dvh flex-1 items-center justify-center p-6">
	<WebAuthGate session={data.session} returnPath={page.url.pathname + page.url.search}>
		<Card.Root class="mx-auto w-full max-w-sm">
			<Card.Header>
				<Card.Title class="text-2xl">{text.title}</Card.Title>
				{#if !authorizationID}
					<Card.Description>{text.missing}</Card.Description>
				{:else if request === null}
					<Card.Description>{text.reading}</Card.Description>
				{:else if request.kind === 'unreadable'}
					<Card.Description>{text.unreadable}</Card.Description>
				{:else if request.kind === 'decided'}
					<Card.Description>{text.redirecting}</Card.Description>
				{:else}
					<Card.Description>
						{text.asking
							.replace('{client}', request.details.client.name || hostOf(request.details.redirect_uri))
							.replace('{email}', request.details.user.email)}
					</Card.Description>
				{/if}
			</Card.Header>
			{#if request?.kind === 'asking'}
				<Card.Content class="space-y-4">
					<p class="text-sm">{text.grants}</p>
					<Field.Field>
						<Field.Label for="connected-app-permission">{text.permission}</Field.Label>
						<Select.Root type="single" bind:value={permission} disabled={isAnswering}>
							<Select.Trigger id="connected-app-permission">{permissionLabels[permission]}</Select.Trigger>
							<Select.Content>
								{#each connectedAppPermissions as choice (choice)}
									<Select.Item value={choice} label={permissionLabels[choice]}>{permissionLabels[choice]}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</Field.Field>
					{#if returnsToThisComputer(request.details.redirect_uri)}
						<p class="text-sm text-muted-foreground">
							{text.returnsTo.replace('{host}', hostOf(request.details.redirect_uri))}
						</p>
					{:else}
						<Alert.Root variant="destructive">
							<TriangleAlertIcon />
							<Alert.Description>
								{text.leavesThisComputer.replace('{host}', hostOf(request.details.redirect_uri))}
							</Alert.Description>
						</Alert.Root>
					{/if}
					<p class="text-sm text-muted-foreground">{text.notStartedByYou}</p>
					{#if failure}
						<p class="text-sm text-destructive">{failure}</p>
					{/if}
					<div class="flex gap-2">
						<Button class="flex-1" variant="outline" disabled={isAnswering} onclick={() => answer(false)}>
							{text.deny}
						</Button>
						<Button class="flex-1" disabled={isAnswering} onclick={() => answer(true)}>
							{text.approve}
						</Button>
					</div>
				</Card.Content>
			{/if}
		</Card.Root>
	</WebAuthGate>
</main>

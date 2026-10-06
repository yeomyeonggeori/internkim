<script lang="ts">
	import { Spinner } from '$lib/components/ui/spinner';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { haveIConnectedMyMessenger } from '$lib/messenger/messenger-directory';
	import {
		connectMessengerAccount,
		whatTheMessengerNeeds,
		type CredentialRequirement
	} from '$lib/messenger/messenger-account';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let requirement = $state<CredentialRequirement | null>(null);
	let answers = $state<Record<string, string>>({});
	let connectedAs = $state('');
	let hasConnected = $state(false);
	let isLoading = $state(true);
	let isConnecting = $state(false);

	onMount(async () => {
		await Promise.all([readWhetherIAmConnected(), readWhatIsNeeded()]);
		isLoading = false;
	});

	async function readWhetherIAmConnected() {
		hasConnected = await haveIConnectedMyMessenger();
	}

	async function readWhatIsNeeded() {
		try {
			requirement = await whatTheMessengerNeeds();
			answers = Object.fromEntries(requirement.fields.map((field) => [field.name, '']));
		} catch {
			requirement = null;
		}
	}

	async function connect() {
		isConnecting = true;
		try {
			const identity = await connectMessengerAccount(answers);
			connectedAs = identity.name || identity.externalID;
			hasConnected = true;
			answers = Object.fromEntries(Object.keys(answers).map((field) => [field, '']));
			toast.success(text.messengerConnected);
		} catch (refusal) {
			toast.error(refusal instanceof Error ? refusal.message : text.messengerConnectFailed);
		} finally {
			isConnecting = false;
		}
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title class="flex items-center gap-2">
			<MessageSquareIcon class="size-4 text-muted-foreground" />
			{text.myMessenger}
		</Card.Title>
		<Card.Description>{text.myMessengerDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if isLoading}
			<p role="status" class="flex items-center gap-2 text-sm text-muted-foreground"><Spinner />{text.messengerLoading}</p>
		{:else if !requirement}
			<p class="text-sm text-muted-foreground">{text.messengerUnavailable}</p>
		{:else}
			{#if connectedAs}
				<p class="text-sm text-muted-foreground">
					{text.messengerConnectedAs.replace('{name}', connectedAs)}
				</p>
			{:else if hasConnected}
				<p class="text-sm text-muted-foreground">{text.messengerAlreadyConnected}</p>
			{/if}
			{#each requirement.fields as field (field.name)}
				<div class="grid gap-2">
					<Label for="{field.name}-{fieldID}">{field.label}</Label>
					<Input
						id="{field.name}-{fieldID}"
						type={field.isSecret ? 'password' : 'text'}
						bind:value={answers[field.name]}
						autocomplete={field.isSecret ? 'current-password' : 'username'}
					/>
				</div>
			{/each}
			<div>
				<Button onclick={connect} disabled={isConnecting}>
					{hasConnected ? text.messengerReconnect : text.messengerConnect}
				</Button>
			</div>
		{/if}
	</Card.Content>
</Card.Root>

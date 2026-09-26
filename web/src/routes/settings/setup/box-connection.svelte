<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { boxStepOf, shortBoxName } from './box-step';
	import { connectBox, fetchBoxes, giveBoxModelKey, type Boxes } from './host-setup-client';
	import { hostSetupText } from './text';

	const text = createPageText(hostSetupText);
	const refreshMilliseconds = 5000;
	let boxes = $state<Boxes>({ connected: null, empty: [] });
	let modelKey = $state('');
	let isChangingModelKey = $state(false);
	let connectingKey = $state('');
	let isSendingModelKey = $state(false);
	let errorMessage = $state('');
	let refreshTimer: ReturnType<typeof setInterval> | undefined;
	const step = $derived(boxStepOf(boxes));

	onMount(() => {
		void refresh();
		refreshTimer = setInterval(() => {
			if (step !== 'connected') void refresh();
		}, refreshMilliseconds);
	});

	onDestroy(() => clearInterval(refreshTimer));

	async function refresh() {
		try {
			boxes = await fetchBoxes();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		}
	}

	async function connect(publicKey: string) {
		connectingKey = publicKey;
		errorMessage = '';
		try {
			boxes = { connected: await connectBox(publicKey), empty: [] };
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			connectingKey = '';
		}
	}

	async function sendModelKey(event: SubmitEvent) {
		event.preventDefault();
		if (!boxes.connected) return;
		if (!modelKey.trim()) {
			errorMessage = text.modelKeyMissing;
			return;
		}
		isSendingModelKey = true;
		errorMessage = '';
		try {
			boxes = { ...boxes, connected: await giveBoxModelKey(boxes.connected, modelKey) };
			modelKey = '';
			isChangingModelKey = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			isSendingModelKey = false;
		}
	}
</script>

<div class="grid min-w-0 gap-4">
	{#if step === 'searching'}
		<div role="status" class="grid gap-1">
			<p class="flex items-center gap-2 text-sm font-medium"><Spinner />{text.searching}</p>
			<p class="text-sm text-muted-foreground">{text.searchingHint}</p>
		</div>
	{:else if step === 'choosing'}
		<Item.Group class="gap-2">
			{#each boxes.empty as box (box.publicKey)}
				<Item.Root variant="outline">
					<Item.Content>
						<Item.Title>{text.foundBox}</Item.Title>
						<Item.Description class="font-mono">{shortBoxName(box.publicKey)}</Item.Description>
					</Item.Content>
					<Item.Actions>
						<Button onclick={() => connect(box.publicKey)} disabled={connectingKey !== ''}>
							{connectingKey === box.publicKey ? text.connecting : text.connect}
						</Button>
					</Item.Actions>
				</Item.Root>
			{/each}
		</Item.Group>
	{:else if boxes.connected}
		<Item.Root variant="outline">
			<Item.Content>
				<Item.Title>{text.foundBox}</Item.Title>
				<Item.Description class="font-mono">{shortBoxName(boxes.connected.publicKey)}</Item.Description>
			</Item.Content>
			{#if step === 'connected' && !isChangingModelKey}
				<Item.Actions>
					<Button variant="outline" onclick={() => (isChangingModelKey = true)}>{text.changeModelKey}</Button>
				</Item.Actions>
			{/if}
		</Item.Root>
		<p role="status" class="text-sm">{step === 'connected' ? text.boxConnected : text.boxClaimed}</p>
	{/if}

	{#if step === 'givingModelKey' || isChangingModelKey}
		<form onsubmit={sendModelKey}>
			<Field.Field>
				<Field.Label for="box-model-key">{text.modelKey}</Field.Label>
				<div class="flex flex-wrap items-center gap-2">
					<Input id="box-model-key" class="min-w-64 flex-1" bind:value={modelKey} type="password" autocomplete="off" />
					<Button type="submit" disabled={isSendingModelKey}>
						{isSendingModelKey ? text.sendingModelKey : text.sendModelKey}
					</Button>
				</div>
				<Field.Description>
					{text.modelKeyDescription}
					<a class="underline" href="https://openrouter.ai/keys" target="_blank" rel="noreferrer">{text.modelKeyLink}</a>
				</Field.Description>
			</Field.Field>
		</form>
	{/if}

	{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
</div>

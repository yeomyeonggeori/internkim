<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Field from '$lib/components/ui/field';
	import { Switch } from '$lib/components/ui/switch';
	import { saveCompanySettings } from '$lib/company/company-settings';
	import type { CompanyBoxes } from '$lib/company/box';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { fetchBoxes, forgetModelKey } from './host-setup-client';
	import { hostSetupText } from './text';

	let {
		boxes,
		onChanged,
		onChangeModelKey
	}: { boxes: CompanyBoxes; onChanged: (boxes: CompanyBoxes) => void; onChangeModelKey: (() => void) | null } = $props();

	const text = createPageText(hostSetupText);
	let usesOwnModelKey = $derived(boxes.usesOwnModelKey);
	let isSaving = $state(false);
	let errorMessage = $state('');

	const payingNow = $derived(
		{ company_key: text.paidByCompanyKey, subscription: text.paidBySubscription, nothing: text.paidByNothing, unknown: text.paidByUnknown }[boxes.paidBy]
	);

	async function chooseWhoPays(chosen: boolean) {
		isSaving = true;
		errorMessage = '';
		try {
			await saveCompanySettings({ usesOwnModelKey: chosen });
			onChanged(await fetchBoxes());
		} catch (error) {
			usesOwnModelKey = boxes.usesOwnModelKey;
			errorMessage = error instanceof Error ? error.message : text.usesOwnModelKeyFailed;
		} finally {
			isSaving = false;
		}
	}

	async function removeKey() {
		errorMessage = '';
		try {
			onChanged(await forgetModelKey());
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.removeModelKeyFailed;
		}
	}

	function askToRemoveKey() {
		confirmDelete({
			title: text.removeModelKeyTitle,
			description: text.removeModelKeyDescription,
			confirm: { text: text.removeModelKey },
			cancel: { text: text.cancel },
			onConfirm: removeKey
		});
	}
</script>

<Field.Set>
	<Field.Legend>{text.modelPayer}</Field.Legend>
	<p role="status" class="text-sm">{payingNow}</p>
	<Field.Field orientation="horizontal">
		<Field.Content>
			<Field.Label for="uses-own-model-key">{text.usesOwnModelKey}</Field.Label>
			<Field.Description>{text.usesOwnModelKeyDescription}</Field.Description>
		</Field.Content>
		<Switch id="uses-own-model-key" bind:checked={usesOwnModelKey} onCheckedChange={chooseWhoPays} disabled={isSaving} />
	</Field.Field>
	{#if boxes.isPayerChangeWaiting}<p class="text-sm text-muted-foreground">{text.payerChangeWaiting}</p>{/if}
	{#if onChangeModelKey || boxes.hasModelKey}
		<div class="flex flex-wrap gap-2">
			{#if onChangeModelKey}<Button variant="outline" onclick={onChangeModelKey}>{text.changeModelKey}</Button>{/if}
			{#if boxes.hasModelKey}<Button variant="outline" onclick={askToRemoveKey}>{text.removeModelKey}</Button>{/if}
		</div>
	{/if}
	{#if !boxes.hasModelKey && boxes.isModelKeyOnlyOnTheComputer}
		<p class="text-sm text-muted-foreground">{text.modelKeyOnTheComputer}</p>
	{/if}
	{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
</Field.Set>

<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Field from '$lib/components/ui/field';
	import type { CompanyBoxes } from '$lib/company/box';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { forgetModelKey } from './host-setup-client';
	import { hostSetupText } from './text';

	let {
		boxes,
		onChanged,
		onChangeModelKey
	}: { boxes: CompanyBoxes; onChanged: (boxes: CompanyBoxes) => void; onChangeModelKey: (() => void) | null } = $props();

	const text = createPageText(hostSetupText);
	let errorMessage = $state('');

	const payingNow = $derived(
		{ company_key: text.paidByCompanyKey, subscription: text.paidBySubscription, nothing: text.paidByNothing, unknown: text.paidByUnknown }[boxes.paidBy]
	);

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
	<Field.Description>{text.payerRule}</Field.Description>
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

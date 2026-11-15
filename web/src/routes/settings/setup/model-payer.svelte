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

	let { boxes, onChanged }: { boxes: CompanyBoxes; onChanged: (boxes: CompanyBoxes) => void } = $props();

	const text = createPageText(hostSetupText);
	let isSaving = $state(false);
	let errorMessage = $state('');

	const payingNow = $derived({ company_key: text.paidByCompanyKey, subscription: text.paidBySubscription, nothing: text.paidByNothing }[boxes.paidBy]);

	async function chooseWhoPays(usesOwnModelKey: boolean) {
		isSaving = true;
		errorMessage = '';
		try {
			await saveCompanySettings({ usesOwnModelKey });
			onChanged(await fetchBoxes());
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.usesOwnModelKeyFailed;
		} finally {
			isSaving = false;
		}
	}

	function askToRemoveKey() {
		confirmDelete({
			title: text.removeModelKeyTitle,
			description: text.removeModelKeyDescription,
			confirm: { text: text.removeModelKey },
			cancel: { text: text.cancel },
			onConfirm: async () => onChanged(await forgetModelKey())
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
		<Switch id="uses-own-model-key" checked={boxes.usesOwnModelKey} onCheckedChange={chooseWhoPays} disabled={isSaving} />
	</Field.Field>
	{#if boxes.hasModelKey}
		<Button variant="outline" class="justify-self-start" onclick={askToRemoveKey}>{text.removeModelKey}</Button>
	{/if}
	{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
</Field.Set>

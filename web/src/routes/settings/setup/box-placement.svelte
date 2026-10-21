<script lang="ts">
	import { untrack } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { CompanyBoxes, ConnectedBox } from '$lib/company/box';
	import { placeBox } from './host-setup-client';
	import { hostSetupText } from './text';

	let { box, onPlaced }: { box: ConnectedBox; onPlaced: (boxes: CompanyBoxes) => void } = $props();

	const text = createPageText(hostSetupText);
	let site = $state(untrack(() => box.site));
	let isSaving = $state(false);
	let errorMessage = $state('');
	const fieldID = $derived(`box-placement-${box.publicKey}`);

	async function save(event: SubmitEvent) {
		event.preventDefault();
		isSaving = true;
		errorMessage = '';
		try {
			onPlaced(await placeBox(box, { site }));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			isSaving = false;
		}
	}
</script>

<Collapsible.Root class="grid gap-2">
	<Collapsible.Trigger class="inline-flex min-h-11 items-center justify-self-start text-sm underline">{text.placement}</Collapsible.Trigger>
	<Collapsible.Content>
		<form class="grid gap-3" onsubmit={save}>
			<Field.Field>
				<Field.Label for="{fieldID}-site">{text.site}</Field.Label>
				<Input id="{fieldID}-site" class="w-full min-w-0 sm:max-w-64" bind:value={site} autocomplete="off" />
				<Field.Description>{text.siteDescription}</Field.Description>
			</Field.Field>
			<Button type="submit" class="justify-self-start" disabled={isSaving}>{isSaving ? text.savingPlacement : text.savePlacement}</Button>
			{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
		</form>
	</Collapsible.Content>
</Collapsible.Root>

<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Select from '$lib/components/ui/select';
	import { Label } from '$lib/components/ui/label';
	import { loadCompanyBaseCurrency, saveCompanyBaseCurrency } from '$lib/company/base-currency';
	import {
		interimCurrencyCatalogue,
		loadCurrencyCatalogue,
		type CurrencyCatalogue
	} from '$lib/currency/currency-catalogue';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let catalogue = $state<CurrencyCatalogue>(interimCurrencyCatalogue);
	let saved = $state('');
	let chosen = $state('');
	let isLoading = $state(true);
	let isSaving = $state(false);

	const hasChange = $derived(chosen !== '' && chosen !== saved);
	const chosenName = $derived(catalogue.find((entry) => entry.code === chosen)?.name ?? chosen);

	onMount(async () => {
		try {
			[catalogue, saved] = await Promise.all([loadCurrencyCatalogue(), loadCompanyBaseCurrency()]);
			chosen = saved;
		} catch {
			toast.error(text.baseCurrencyLoadFailed);
		} finally {
			isLoading = false;
		}
	});

	async function save() {
		isSaving = true;
		try {
			await saveCompanyBaseCurrency(chosen);
			saved = chosen;
			toast.success(text.saved);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.saveFailed);
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.baseCurrency}</Card.Title>
		<Card.Description>{text.baseCurrencyDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if !isLoading}
			<div class="grid gap-2">
				<Label for="{fieldID}-currency">{text.baseCurrency}</Label>
				<Select.Root type="single" value={chosen} onValueChange={(value) => (chosen = value)} disabled={isSaving}>
					<Select.Trigger id="{fieldID}-currency" class="w-full">{chosen} · {chosenName}</Select.Trigger>
					<Select.Content>
						{#each catalogue as option (option.code)}
							<Select.Item value={option.code} label="{option.code} · {option.name}">
								{option.code} · {option.name}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			<p class="text-sm text-muted-foreground">{text.baseCurrencySettledNote}</p>
			<Button class="w-full" onclick={save} disabled={!hasChange || isSaving}>{text.save}</Button>
		{/if}
	</Card.Content>
</Card.Root>

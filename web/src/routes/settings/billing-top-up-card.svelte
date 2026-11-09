<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { creditTopUpSetting, saveCreditTopUpSetting, type CreditTopUpSetting } from '$lib/company/credits';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { billingText } from './billing-text';

	const text = createPageText(billingText);
	const fieldID = $props.id();

	let saved = $state<CreditTopUpSetting | null>(null);
	let enabled = $state(false);
	let thresholdCredits = $state(5000);
	let amount = $state(20000);
	let isLoading = $state(true);
	let isSaving = $state(false);

	const hasChange = $derived(
		saved === null
			? enabled
			: saved.enabled !== enabled || saved.thresholdCredits !== thresholdCredits || saved.amount !== amount
	);
	const isValid = $derived(Number.isInteger(thresholdCredits) && thresholdCredits > 0 && Number.isInteger(amount) && amount > 0);

	onMount(async () => {
		try {
			saved = await creditTopUpSetting();
			if (saved) ({ enabled, thresholdCredits, amount } = saved);
		} catch {
			toast.error(text.loadFailed);
		} finally {
			isLoading = false;
		}
	});

	async function save() {
		isSaving = true;
		try {
			const setting = { enabled, thresholdCredits, amount };
			await saveCreditTopUpSetting(setting);
			saved = setting;
			toast.success(text.saved);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.saveFailed);
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header class="flex items-start justify-between gap-4">
		<div class="grid gap-1">
			<Card.Title>{text.topUp}</Card.Title>
			<Card.Description>{text.topUpDescription}</Card.Description>
		</div>
		<Switch bind:checked={enabled} disabled={isLoading || isSaving} aria-label={text.topUp} />
	</Card.Header>
	<Card.Content class="grid gap-4">
		<div class="grid gap-4 sm:grid-cols-2">
			<div class="grid gap-2">
				<Label for="{fieldID}-threshold">{text.topUpThreshold}</Label>
				<Input id="{fieldID}-threshold" type="number" min="1" bind:value={thresholdCredits} disabled={isLoading || isSaving || !enabled} />
			</div>
			<div class="grid gap-2">
				<Label for="{fieldID}-amount">{text.topUpAmount}</Label>
				<Input id="{fieldID}-amount" type="number" min="1" bind:value={amount} disabled={isLoading || isSaving || !enabled} />
			</div>
		</div>
		<Button class="w-full" onclick={save} disabled={isLoading || isSaving || !hasChange || !isValid}>{text.save}</Button>
	</Card.Content>
</Card.Root>

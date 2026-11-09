<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Meter } from '$lib/components/ui/meter';
	import { saveCreditUsageLimit, type CompanyCredits } from '$lib/company/credits';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { toast } from 'svelte-sonner';
	import { billingText } from './billing-text';
	import { creditCount, filled } from './billing-format';

	let { credits, onchange }: { credits: CompanyCredits; onchange: () => void } = $props();

	const text = createPageText(billingText);
	const fieldID = $props.id();
	const nearLimitShare = 0.8;

	let limit = $state(0);
	let isSaving = $state(false);

	$effect.pre(() => {
		limit = credits.usageLimit;
	});

	const isValid = $derived(Number.isInteger(limit) && limit > 0);

	async function save() {
		isSaving = true;
		try {
			await saveCreditUsageLimit(limit);
			toast.success(text.saved);
			onchange();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.saveFailed);
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.memberUsage}</Card.Title>
		<Card.Description>{text.memberUsageDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-5">
		<div class="flex items-end gap-2">
			<div class="grid gap-2">
				<Label for="{fieldID}-limit">{text.usageLimit}</Label>
				<Input id="{fieldID}-limit" type="number" min="1" class="w-32" bind:value={limit} disabled={isSaving} />
			</div>
			<Button variant="outline" onclick={save} disabled={isSaving || !isValid || limit === credits.usageLimit}>{text.save}</Button>
		</div>
		<ul class="grid gap-3">
			{#each credits.members as member (member.personID)}
				{@const isNearLimit = member.creditsUsed >= credits.usageLimit * nearLimitShare}
				<li class="grid grid-cols-[6rem_minmax(0,1fr)_7rem] items-center gap-3 text-sm">
					<span class="truncate font-medium">{member.name}</span>
					<Meter
						value={Math.min(member.creditsUsed, credits.usageLimit)}
						max={credits.usageLimit}
						aria-label={member.name}
						class={isNearLimit ? '[--meter-background:var(--destructive)]' : ''}
					/>
					<span class="text-right tabular-nums {isNearLimit ? 'text-destructive' : 'text-muted-foreground'}">
						{filled(text.memberUsed, {
							used: creditCount(member.creditsUsed, currentLocale.value),
							limit: creditCount(credits.usageLimit, currentLocale.value)
						})}
					</span>
				</li>
			{/each}
		</ul>
	</Card.Content>
</Card.Root>

<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import EraserIcon from '@lucide/svelte/icons/eraser';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Field from '$lib/components/ui/field';
	import { forgetMemoryFact, type MemoryFact } from './memory-facts-api';
	import { isLiveMemoryFact, memoryAudience, memoryDate, memoryKindLabel } from './memory-workbench-model';
	import type { MemoryText } from './text';

	let { fact, text, onForgotten }: { fact: MemoryFact; text: MemoryText; onForgotten: (factID: string) => void } = $props();
	let isForgetting = $state(false);
	let isConfirmingForget = $state(false);
	let reason = $state('');
	let errorMessage = $state('');
	const isLive = $derived(isLiveMemoryFact(fact));

	function beginForget(): void {
		reason = '';
		errorMessage = '';
		isConfirmingForget = true;
	}

	async function forget(): Promise<void> {
		if (isForgetting) return;
		isForgetting = true;
		errorMessage = '';
		try {
			await forgetMemoryFact(fact.factID, reason);
			isConfirmingForget = false;
			onForgotten(fact.factID);
		} catch { errorMessage = text.forgetFailed; }
		finally { isForgetting = false; }
	}
</script>

<div class="flex min-w-0 flex-col gap-5 px-5 py-5 sm:px-6">
	<header class="flex flex-col gap-3">
		<h2 class="text-sm font-semibold">{text.memoryDetails}</h2>
		<div class="flex flex-wrap items-center gap-2">
			<Badge variant="secondary">{memoryKindLabel(fact.kind, text)}</Badge>
			<Badge variant="secondary">{memoryAudience(fact, text)}</Badge>
			<Badge variant="outline">{isLive ? text.current : text.previousMemory}</Badge>
		</div>
	</header>
	{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
	<div class="memory-markdown break-words text-base leading-7"><SvelteMarkdown source={fact.content} /></div>
	{#if isConfirmingForget}
		<form class="grid gap-4 rounded-lg border bg-muted/30 p-4" aria-label={text.forgetTitle} onsubmit={(event) => { event.preventDefault(); void forget(); }}>
			<div class="grid gap-1"><h3 class="text-sm font-medium">{text.forgetTitle}</h3><p class="text-sm text-muted-foreground">{text.forgetDescription}</p></div>
			<Field.FieldGroup>
			<Field.Field>
				<Field.FieldLabel for="memory-forget-reason">{text.forgetReasonLabel}</Field.FieldLabel>
				<Input id="memory-forget-reason" maxlength={200} bind:value={reason} placeholder={text.forgetReasonPlaceholder} disabled={isForgetting} />
			</Field.Field>
			</Field.FieldGroup>
			<div class="flex gap-2"><Button type="submit" variant="destructive" disabled={isForgetting}>{#if isForgetting}<Spinner data-icon="inline-start" />{/if}{text.forgetAction}</Button><Button type="button" variant="outline" disabled={isForgetting} onclick={() => isConfirmingForget = false}>{text.cancel}</Button></div>
		</form>
	{:else if isLive}
		<div class="flex gap-2"><Button variant="outline" size="sm" onclick={beginForget}><EraserIcon data-icon="inline-start" />{text.forget}</Button></div>
	{/if}
	<Separator />
	<dl class="grid grid-cols-2 gap-4 text-sm">
		<div class="grid gap-1"><dt class="text-xs text-muted-foreground">{fact.validUntil ? text.validity : text.validFrom}</dt><dd>{memoryDate(fact.validFrom, text, currentLocale.value)}{#if fact.validUntil} → {memoryDate(fact.validUntil, text, currentLocale.value)}{/if}</dd></div>
		<div class="grid gap-1"><dt class="text-xs text-muted-foreground">{text.circles}</dt><dd>{fact.circleIDs.length > 0 ? fact.circleIDs.join(', ') : text.myMemory}</dd></div>
		<div class="grid gap-1"><dt class="text-xs text-muted-foreground">{text.reinforcementCount}</dt><dd>{text.reinforcementCountTemplate.replace('{count}', String(fact.reinforcementCount))}</dd></div>
		<div class="grid gap-1"><dt class="text-xs text-muted-foreground">{text.lastRecalledAt}</dt><dd>{fact.lastRecalledAt ? memoryDate(fact.lastRecalledAt, text, currentLocale.value) : text.neverRecalled}</dd></div>
	</dl>
	<section class="grid gap-2" aria-label={text.triggerPhrases}>
		<h3 class="text-xs text-muted-foreground">{text.triggerPhrases}</h3>
		{#if fact.triggerPhrases.length > 0}
			<div class="flex flex-wrap gap-2">
				{#each fact.triggerPhrases as phrase (phrase)}<Badge variant="outline">{phrase}</Badge>{/each}
			</div>
			<p class="text-xs text-muted-foreground">{text.triggerPhrasesDescription}</p>
		{:else}
			<p class="text-sm text-muted-foreground">{text.triggerPhrasesPending}</p>
		{/if}
	</section>
</div>

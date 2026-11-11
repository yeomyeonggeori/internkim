<script lang="ts">
	import * as Item from '$lib/components/ui/item/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
	import CheckIcon from '@lucide/svelte/icons/check';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChoiceCard, ChoiceCardOption } from './choice-card';

	let { card, onChoose }: { card: ChoiceCard; onChoose: (option: ChoiceCardOption) => void } = $props();

	const text = createPageText(channelText);
</script>

{#snippet listOptionContent(option: ChoiceCardOption)}
	<Item.Media>
		<Badge variant={option.look === 'chosen' ? 'default' : 'outline'} class="tabular-nums">{option.position}</Badge>
	</Item.Media>
	<Item.Content class="min-w-0 break-words">{option.label}</Item.Content>
	{#if option.look === 'chosen'}
		<Item.Actions>
			<CheckIcon class="text-brand size-4" />
			<span class="sr-only">{text.choiceChosen}</span>
		</Item.Actions>
	{:else if card.isOpen && option.isRecommended}
		<Item.Actions>
			<Badge variant="secondary">{text.choiceRecommended}</Badge>
		</Item.Actions>
	{/if}
{/snippet}

{#snippet listOption(option: ChoiceCardOption)}
	<div role="listitem">
		<Item.Root
			variant="outline"
			size="xs"
			data-look={option.look}
			class={[option.look === 'faded' && 'text-muted-foreground', option.look === 'pressable' && 'hover:bg-muted text-left']}
		>
			{#snippet child({ props })}
				{#if option.look === 'pressable'}
					<button {...props} type="button" onclick={() => onChoose(option)}>
						{@render listOptionContent(option)}
					</button>
				{:else}
					<div {...props}>
						{@render listOptionContent(option)}
					</div>
				{/if}
			{/snippet}
		</Item.Root>
	</div>
{/snippet}

{#snippet pairOption(option: ChoiceCardOption)}
	{@const variant = option.look === 'faded' ? 'outline' : option.isDeclining ? 'destructive' : 'default'}
	{#if option.look === 'chosen'}
		<span class={buttonVariants({ variant, class: 'flex-1' })} data-look={option.look}>
			<CheckIcon />
			<span class="sr-only">{text.choiceChosen}</span>
			{option.label}
		</span>
	{:else}
		<Button
			{variant}
			class="flex-1"
			disabled={option.look !== 'pressable'}
			data-look={option.look}
			onclick={() => onChoose(option)}
		>
			{option.label}
		</Button>
	{/if}
{/snippet}

<div class="flex w-full max-w-sm flex-col gap-1.5 self-start" data-slot="choice-card">
	{#if card.layout === 'pair'}
		<div class="flex gap-2">
			{#each card.options as option (option.key)}
				{@render pairOption(option)}
			{/each}
		</div>
	{:else}
		<Item.Group>
			{#each card.options as option (option.key)}
				{@render listOption(option)}
			{/each}
		</Item.Group>
	{/if}
</div>

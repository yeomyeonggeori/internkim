<script lang="ts">
	import * as Item from '$lib/components/ui/item/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ChoiceCard, ChoiceCardOption } from './choice-card';

	let { card, onChoose }: { card: ChoiceCard; onChoose: (option: ChoiceCardOption) => void } = $props();

	const text = createPageText(channelText);
</script>

{#snippet listOptionContent(option: ChoiceCardOption)}
	<Item.Media>
		<Badge variant="outline" class="tabular-nums">{option.position}</Badge>
	</Item.Media>
	<Item.Content class="min-w-0 break-words">{option.label}</Item.Content>
	{#if option.isRecommended}
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
			class={[option.look === 'pressable' && 'hover:bg-muted text-left']}
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
	<Button
		variant={option.isDeclining ? 'destructive' : 'default'}
		class="flex-1"
		disabled={option.look !== 'pressable'}
		data-look={option.look}
		onclick={() => onChoose(option)}
	>
		{option.label}
	</Button>
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

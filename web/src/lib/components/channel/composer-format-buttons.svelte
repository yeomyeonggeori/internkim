<script lang="ts">
	import * as ButtonGroup from '$lib/components/ui/button-group/index.js';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { cn } from '$lib/utils';
	import type { ComposerFormat } from './composer-formatting';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import BoldIcon from '@lucide/svelte/icons/bold';
	import CodeIcon from '@lucide/svelte/icons/code';
	import ItalicIcon from '@lucide/svelte/icons/italic';
	import LinkIcon from '@lucide/svelte/icons/link';
	import ListIcon from '@lucide/svelte/icons/list';
	import ListOrderedIcon from '@lucide/svelte/icons/list-ordered';
	import StrikethroughIcon from '@lucide/svelte/icons/strikethrough';
	import TextQuoteIcon from '@lucide/svelte/icons/text-quote';
	import type { Component } from 'svelte';

	let { disabled, activeFormats = [], onFormat, onClear, onPointerdown }: { disabled: boolean; activeFormats?: ComposerFormat[]; onFormat: (format: ComposerFormat) => void; onClear?: () => void; onPointerdown?: (event: PointerEvent) => void } = $props();

	const text = createPageText(channelText);

	type FormatButton = { format: ComposerFormat; label: string; icon: Component };

	const groups = $derived<FormatButton[][]>([
		[
			{ format: 'bold', label: text.formatBold, icon: BoldIcon },
			{ format: 'italic', label: text.formatItalic, icon: ItalicIcon },
			{ format: 'strikethrough', label: text.formatStrikethrough, icon: StrikethroughIcon },
			{ format: 'link', label: text.formatLink, icon: LinkIcon }
		],
		[
			{ format: 'orderedList', label: text.formatOrderedList, icon: ListOrderedIcon },
			{ format: 'bulletList', label: text.formatBulletList, icon: ListIcon },
			{ format: 'quote', label: text.formatQuote, icon: TextQuoteIcon },
			{ format: 'code', label: text.formatCode, icon: CodeIcon }
		]
	]);
</script>

<ButtonGroup.Root class="flex-wrap gap-1" aria-label={text.selectionFormatting}>
	{#each groups as group, index (index)}
		<ButtonGroup.Root class={cn(onClear && 'w-full')}>
			{#each group as button (button.format)}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<InputGroup.Button
								{...props}
								size="icon-xs"
								variant={activeFormats.includes(button.format) ? 'secondary' : onClear ? 'outline' : 'ghost'}
								class={cn(onClear && 'flex-1')}
								onpointerdown={onPointerdown}
								aria-label={button.label}
								aria-pressed={activeFormats.includes(button.format)}
								{disabled}
								onclick={() => onFormat(button.format)}
							>
								<button.icon />
							</InputGroup.Button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content side="top">{button.label}</Tooltip.Content>
				</Tooltip.Root>
			{/each}
		</ButtonGroup.Root>
	{/each}
	{#if onClear}
		<ButtonGroup.Root class="w-full">
			<InputGroup.Button variant="outline" class="w-full" {disabled} onpointerdown={onPointerdown} onclick={onClear}>{text.clearFormatting}</InputGroup.Button>
		</ButtonGroup.Root>
	{/if}
</ButtonGroup.Root>

<script lang="ts">
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import BellIcon from '@lucide/svelte/icons/bell';
	import BellOffIcon from '@lucide/svelte/icons/bell-off';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import FileDownIcon from '@lucide/svelte/icons/file-down';
	import { mergeProps } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { fromAction } from 'svelte/attachments';
	import type { HTMLLiAttributes } from 'svelte/elements';
	import { MediaQuery } from 'svelte/reactivity';
	import { swallowClickAfterTouchHold } from './swallow-click-after-touch-hold';

	type Props = {
		isMuted: boolean;
		muteLabel: string;
		unmuteLabel: string;
		menuLabel: string;
		onSwitchMuted: () => void;
		exportLabel?: string;
		onExport?: () => void;
		itemProps?: HTMLLiAttributes & Record<string, unknown>;
		children: Snippet;
	};

	let {
		isMuted,
		muteLabel,
		unmuteLabel,
		menuLabel,
		onSwitchMuted,
		exportLabel = '',
		onExport,
		itemProps = {},
		children
	}: Props = $props();

	const canHover = new MediaQuery('hover: hover', true);
	let isContextMenuOpen = $state(false);
</script>

{#snippet switchMutedLabel()}
	{#if isMuted}
		<BellIcon />
		<span>{unmuteLabel}</span>
	{:else}
		<BellOffIcon />
		<span>{muteLabel}</span>
	{/if}
{/snippet}

{#snippet exportConversationLabel()}
	<FileDownIcon />
	<span>{exportLabel}</span>
{/snippet}

<ContextMenu.Root bind:open={isContextMenuOpen}>
	<ContextMenu.Trigger class="[-webkit-touch-callout:none]">
		{#snippet child({ props })}
			<Sidebar.MenuItem
				{...mergeProps(props, itemProps)}
				{@attach fromAction(swallowClickAfterTouchHold, () => ({ isMenuOpen: isContextMenuOpen }))}
			>
				{@render children()}
				{#if canHover.current}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props: triggerProps })}
								<Sidebar.MenuAction {...triggerProps} showOnHover aria-label={menuLabel}>
									<EllipsisVerticalIcon />
								</Sidebar.MenuAction>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content side="right" align="start" class="w-52">
							<DropdownMenu.Item onSelect={onSwitchMuted}>
								{@render switchMutedLabel()}
							</DropdownMenu.Item>
							{#if onExport}
								<DropdownMenu.Item onSelect={onExport}>
									{@render exportConversationLabel()}
								</DropdownMenu.Item>
							{/if}
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
			</Sidebar.MenuItem>
		{/snippet}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52">
		<ContextMenu.Item onSelect={onSwitchMuted}>
			{@render switchMutedLabel()}
		</ContextMenu.Item>
		{#if onExport}
			<ContextMenu.Item onSelect={onExport}>
				{@render exportConversationLabel()}
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>

<script lang="ts">
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import BellIcon from '@lucide/svelte/icons/bell';
	import BellOffIcon from '@lucide/svelte/icons/bell-off';
	import DoorOpenIcon from '@lucide/svelte/icons/door-open';
	import EyeOffIcon from '@lucide/svelte/icons/eye-off';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import FileDownIcon from '@lucide/svelte/icons/file-down';
	import PinIcon from '@lucide/svelte/icons/pin';
	import PinOffIcon from '@lucide/svelte/icons/pin-off';
	import { mergeProps } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { fromAction } from 'svelte/attachments';
	import type { HTMLLiAttributes } from 'svelte/elements';
	import { MediaQuery } from 'svelte/reactivity';
	import { holdToReorder, type HoldToReorderMoves } from './hold-to-reorder';
	import { swallowClickAfterTouchHold } from './swallow-click-after-touch-hold';

	type Props = {
		isMuted: boolean;
		muteLabel: string;
		unmuteLabel: string;
		menuLabel: string;
		onSwitchMuted: () => void;
		isPinned?: boolean;
		pinLabel?: string;
		unpinLabel?: string;
		onSwitchPinned?: () => void;
		exportLabel?: string;
		onExport?: () => void;
		leaveLabel?: string;
		onLeave?: () => void;
		hideLabel?: string;
		onHide?: () => void;
		itemProps?: HTMLLiAttributes & Record<string, unknown>;
		holdMoves?: HoldToReorderMoves;
		children: Snippet;
	};

	let {
		isMuted,
		muteLabel,
		unmuteLabel,
		menuLabel,
		onSwitchMuted,
		isPinned = false,
		pinLabel = '',
		unpinLabel = '',
		onSwitchPinned,
		exportLabel = '',
		onExport,
		leaveLabel = '',
		onLeave,
		hideLabel = '',
		onHide,
		itemProps = {},
		holdMoves,
		children
	}: Props = $props();

	const canHover = new MediaQuery('hover: hover', true);
	let isContextMenuOpen = $state(false);
</script>

{#snippet switchPinnedLabel()}
	{#if isPinned}
		<PinOffIcon />
		<span>{unpinLabel}</span>
	{:else}
		<PinIcon />
		<span>{pinLabel}</span>
	{/if}
{/snippet}

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

{#snippet leaveConversationLabel()}
	<DoorOpenIcon />
	<span>{leaveLabel}</span>
{/snippet}

{#snippet hideConversationLabel()}
	<EyeOffIcon />
	<span>{hideLabel}</span>
{/snippet}

<ContextMenu.Root bind:open={isContextMenuOpen}>
	<ContextMenu.Trigger class="[-webkit-touch-callout:none]">
		{#snippet child({ props })}
			<Sidebar.MenuItem
				{...mergeProps(props, itemProps)}
				{@attach fromAction(swallowClickAfterTouchHold, () => ({ isMenuOpen: isContextMenuOpen }))}
				{@attach holdMoves &&
					fromAction(holdToReorder, () => ({
						...holdMoves,
						isMenuOpen: isContextMenuOpen,
						closeMenu: () => (isContextMenuOpen = false)
					}))}
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
							{#if onSwitchPinned}
								<DropdownMenu.Item onSelect={onSwitchPinned}>
									{@render switchPinnedLabel()}
								</DropdownMenu.Item>
							{/if}
							<DropdownMenu.Item onSelect={onSwitchMuted}>
								{@render switchMutedLabel()}
							</DropdownMenu.Item>
							{#if onExport}
								<DropdownMenu.Item onSelect={onExport}>
									{@render exportConversationLabel()}
								</DropdownMenu.Item>
							{/if}
							{#if onLeave}
								<DropdownMenu.Item onSelect={onLeave}>
									{@render leaveConversationLabel()}
								</DropdownMenu.Item>
							{/if}
							{#if onHide}
								<DropdownMenu.Item onSelect={onHide}>
									{@render hideConversationLabel()}
								</DropdownMenu.Item>
							{/if}
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
			</Sidebar.MenuItem>
		{/snippet}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52">
		{#if onSwitchPinned}
			<ContextMenu.Item onSelect={onSwitchPinned}>
				{@render switchPinnedLabel()}
			</ContextMenu.Item>
		{/if}
		<ContextMenu.Item onSelect={onSwitchMuted}>
			{@render switchMutedLabel()}
		</ContextMenu.Item>
		{#if onExport}
			<ContextMenu.Item onSelect={onExport}>
				{@render exportConversationLabel()}
			</ContextMenu.Item>
		{/if}
		{#if onLeave}
			<ContextMenu.Item onSelect={onLeave}>
				{@render leaveConversationLabel()}
			</ContextMenu.Item>
		{/if}
		{#if onHide}
			<ContextMenu.Item onSelect={onHide}>
				{@render hideConversationLabel()}
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>

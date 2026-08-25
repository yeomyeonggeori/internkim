<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import BellIcon from '@lucide/svelte/icons/bell';
	import BellOffIcon from '@lucide/svelte/icons/bell-off';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';

	type Props = {
		isMuted: boolean;
		muteLabel: string;
		unmuteLabel: string;
		menuLabel: string;
		onSwitchMuted: () => void;
	};

	let { isMuted, muteLabel, unmuteLabel, menuLabel, onSwitchMuted }: Props = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Sidebar.MenuAction {...props} showOnHover aria-label={menuLabel}>
				<EllipsisVerticalIcon />
			</Sidebar.MenuAction>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content side="right" align="start" class="w-52">
		<DropdownMenu.Item onSelect={onSwitchMuted}>
			{#if isMuted}
				<BellIcon />
				<span>{unmuteLabel}</span>
			{:else}
				<BellOffIcon />
				<span>{muteLabel}</span>
			{/if}
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>

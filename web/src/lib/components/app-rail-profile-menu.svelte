<script lang="ts">
	import AppRailAttendanceItem from '$lib/components/app-rail-attendance-item.svelte';
	import BuzzConnectDialog from '$lib/components/buzz/buzz-connect-dialog.svelte';
	import { attendanceClock } from '$lib/components/attendance-clock.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { AppRailProfileMenuLabels } from '$lib/components/app-rail-types';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Kbd } from '$lib/components/ui/kbd';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { mergeProps } from 'bits-ui';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import SmartphoneIcon from '@lucide/svelte/icons/smartphone';

	let buzzConnectOpen = $state(false);

	let {
		displayUserName,
		userEmail,
		userImage,
		labels,
		logOut
	}: {
		displayUserName: string;
		userEmail: string;
		userImage?: string;
		labels: AppRailProfileMenuLabels;
		logOut: () => void | Promise<void>;
	} = $props();
</script>

<Sidebar.MenuItem>
	<DropdownMenu.Root bind:open={attendanceClock.isMenuOpen}>
		<DropdownMenu.Trigger>
			{#snippet child({ props: triggerProps })}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props: tooltipProps })}
							<Sidebar.MenuButton
								{...mergeProps(triggerProps, tooltipProps)}
								size="lg"
								aria-label={displayUserName}
								class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
							>
								<PersonAvatar name={displayUserName} email={userEmail} image={userImage ?? ''} class="size-8 rounded-lg" />
								<div class="grid flex-1 text-left text-sm leading-tight">
									<span class="truncate font-medium">{displayUserName}</span>
									<span class="truncate text-xs">{userEmail || labels.activeWorkspace}</span>
								</div>
								<ChevronsUpDownIcon class="ml-auto" />
							</Sidebar.MenuButton>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content side="right">
						{labels.account}
						<Kbd>.</Kbd>
					</Tooltip.Content>
				</Tooltip.Root>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content
			side="right"
			align="end"
			sideOffset={4}
			class="w-(--bits-dropdown-menu-anchor-width) min-w-56 rounded-lg"
			data-app-rail-profile-menu
		>
			<DropdownMenu.Label class="p-0 font-normal">
				<div class="flex items-center gap-2 px-1 py-1.5 text-start text-sm">
					<PersonAvatar name={displayUserName} email={userEmail} image={userImage ?? ''} class="size-8 rounded-lg" />
					<div class="grid flex-1 text-start text-sm leading-tight">
						<span class="truncate font-medium">{displayUserName}</span>
						<span class="truncate text-xs">{userEmail || labels.activeWorkspace}</span>
					</div>
				</div>
			</DropdownMenu.Label>
			<DropdownMenu.Separator />
			<AppRailAttendanceItem />
			<DropdownMenu.Separator />
			<DropdownMenu.Item onclick={() => (buzzConnectOpen = true)}>
				<SmartphoneIcon class="size-4" />
				{labels.buzzConnect}
			</DropdownMenu.Item>
			<DropdownMenu.Separator />
			<DropdownMenu.Item variant="destructive" onclick={logOut}>
				{labels.logOut}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<BuzzConnectDialog bind:open={buzzConnectOpen} />
</Sidebar.MenuItem>

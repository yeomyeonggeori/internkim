<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { AppRailProfileMenuLabels } from '$lib/components/app-rail-types';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import BadgeCheckIcon from '@lucide/svelte/icons/badge-check';
	import BellIcon from '@lucide/svelte/icons/bell';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import SettingsIcon from '@lucide/svelte/icons/settings';

	let {
		open = $bindable(false),
		displayUserName,
		userEmail,
		userImage,
		labels,
		openAPITokenSheet,
		logOut
	}: {
		open?: boolean;
		displayUserName: string;
		userEmail: string;
		userImage?: string;
		labels: AppRailProfileMenuLabels;
		openAPITokenSheet: () => void;
		logOut: () => void | Promise<void>;
	} = $props();

	const profileMenuSideOffset = 6;
</script>

<DropdownMenu.Root bind:open>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				aria-label={displayUserName}
				class="mx-2 flex h-11 w-11 items-center gap-3 overflow-hidden rounded-full text-left transition-[width,background-color] duration-150 ease-out hover:bg-sidebar-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring group-hover:w-[208px] group-focus-within:w-[208px] group-data-[profile-open=true]:w-[208px]"
			>
				<span class="relative flex size-11 shrink-0 items-center justify-center rounded-full bg-background shadow-sm ring-1 ring-sidebar-border">
					<PersonAvatar name={displayUserName} email={userEmail} image={userImage ?? ''} class="size-9 rounded-full" />
					<span class="absolute bottom-0.5 right-0.5 size-2.5 rounded-full border-2 border-sidebar bg-success" aria-hidden="true"></span>
				</span>
				<span class="grid min-w-0 max-w-0 leading-tight opacity-0 transition-[max-width,opacity] duration-150 ease-out group-hover:max-w-[140px] group-hover:opacity-100 group-focus-within:max-w-[140px] group-focus-within:opacity-100 group-data-[profile-open=true]:max-w-[140px] group-data-[profile-open=true]:opacity-100">
					<span class="truncate text-sm font-medium text-sidebar-accent-foreground">{displayUserName}</span>
					<span class="truncate text-xs text-sidebar-foreground/60">{userEmail || labels.activeWorkspace}</span>
				</span>
			</button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content side="right" align="end" sideOffset={profileMenuSideOffset} class="min-w-56" data-app-rail-profile-menu>
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
		<DropdownMenu.Item onclick={() => (location.href = '/admin/')}>
			<BadgeCheckIcon />
			{labels.account}
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={openAPITokenSheet}>
			<SettingsIcon />
			{labels.apiTokens}
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={() => (location.href = '/flow/')}>
			<BellIcon />
			{labels.activity}
		</DropdownMenu.Item>
		<DropdownMenu.Separator />
		<DropdownMenu.Item onclick={logOut}>
			<LogOutIcon />
			{labels.logOut}
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>

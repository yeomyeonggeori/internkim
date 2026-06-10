<script lang="ts">
	import { page } from '$app/state';
	import AccountAPITokenSheet from '$lib/components/account-api-token-sheet.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import BadgeCheckIcon from '@lucide/svelte/icons/badge-check';
	import BellIcon from '@lucide/svelte/icons/bell';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import CogIcon from '@lucide/svelte/icons/cog';
	import ListChecksIcon from '@lucide/svelte/icons/list-checks';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import MailIcon from '@lucide/svelte/icons/mail';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import { onMount } from 'svelte';

	type RailItem = {
		href: string;
		label: string;
		icon: typeof MailIcon;
	};

	let userEmail = $state('');
	const text = createPageText(appShellText);
	let isProfileMenuOpen = $state(false);
	let isAPITokenSheetOpen = $state(false);
	let userName = $state('');
	const displayUserName = $derived(userName || text.workspace);
	const profileMenuSideOffset = 6;

	const apps = $derived<RailItem[]>([
		{ href: '/flow/', label: text.flow, icon: ListChecksIcon },
		{ href: '/memory/', label: text.memory, icon: NetworkIcon },
		{ href: '/calendar/', label: text.calendar, icon: CalendarDaysIcon },
		{ href: '/mail/', label: text.mail, icon: MailIcon },
		{ href: '/attendance/', label: text.attendance, icon: ClipboardCheckIcon }
	]);

	const workspace = $derived<RailItem[]>([
		{ href: '/admin/', label: text.admin, icon: CogIcon }
	]);

	function isActive(href: string) {
		const path = page.url.pathname;
		const base = href.replace(/\/$/, '');
		return path === base || path.startsWith(`${base}/`);
	}

	onMount(loadUser);

	async function loadUser() {
		try {
			const response = await fetch('/admin/api/session', { credentials: 'include' });
			if (!response.ok) {
				await loadWebUser();
				return;
			}
			const session = (await response.json()) as { email?: string; claimedAdminEmail?: string };
			userEmail = session.email || session.claimedAdminEmail || '';
			userName = userEmail ? userEmail.split('@')[0] : '';
		} catch {
			await loadWebUser();
		}
	}

	async function loadWebUser() {
		try {
			const response = await fetch('/auth/session', { credentials: 'include' });
			if (!response.ok) return;
			const session = (await response.json()) as { authenticated?: boolean; email?: string };
			if (!session.authenticated) return;
			userEmail = session.email || '';
			userName = userEmail ? userEmail.split('@')[0] : '';
		} catch {
			userEmail = '';
		}
	}

	async function logOut() {
		try {
			await fetch('/auth/logout', { method: 'POST', credentials: 'include' });
		} finally {
			location.href = '/flow/';
		}
	}
</script>

{#snippet railLink(item: RailItem)}
	{@const Icon = item.icon}
	<a
		href={item.href}
		aria-label={item.label}
		data-active={isActive(item.href)}
		data-sveltekit-preload-data="off"
		data-sveltekit-preload-code="off"
		class="relative mx-2.5 flex h-10 w-10 items-center gap-3 overflow-hidden rounded-md px-2.5 text-sm font-medium text-sidebar-foreground/70 transition-[width,color,background-color] duration-150 ease-out before:absolute before:left-0 before:top-1/2 before:h-6 before:w-0.5 before:-translate-y-1/2 before:rounded-r-full before:bg-sidebar-primary before:opacity-0 before:transition-opacity before:content-[''] hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground data-[active=true]:before:opacity-100 group-hover:w-[204px] group-focus-within:w-[204px] group-data-[profile-open=true]:w-[204px]"
	>
		<Icon class="size-5 shrink-0" />
		<span class="min-w-0 max-w-0 truncate opacity-0 transition-[max-width,opacity] duration-150 ease-out group-hover:max-w-[148px] group-hover:opacity-100 group-focus-within:max-w-[148px] group-focus-within:opacity-100 group-data-[profile-open=true]:max-w-[148px] group-data-[profile-open=true]:opacity-100">{item.label}</span>
	</a>
{/snippet}

{#snippet profileMenuContent()}
	<DropdownMenu.Content side="right" align="end" sideOffset={profileMenuSideOffset} class="min-w-56" data-app-rail-profile-menu>
		<DropdownMenu.Label class="p-0 font-normal">
			<div class="flex items-center gap-2 px-1 py-1.5 text-start text-sm">
				<PersonAvatar name={displayUserName} email={userEmail} class="size-8 rounded-lg" />
				<div class="grid flex-1 text-start text-sm leading-tight">
					<span class="truncate font-medium">{displayUserName}</span>
					<span class="truncate text-xs">{userEmail || text.activeWorkspace}</span>
				</div>
			</div>
		</DropdownMenu.Label>
		<DropdownMenu.Separator />
		<DropdownMenu.Item onclick={() => (location.href = '/admin/')}>
			<BadgeCheckIcon />
			{text.account}
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={() => (isAPITokenSheetOpen = true)}>
			<SettingsIcon />
			{text.apiTokens}
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={() => (location.href = '/flow/')}>
			<BellIcon />
			{text.activity}
		</DropdownMenu.Item>
		<DropdownMenu.Separator />
		<DropdownMenu.Item onclick={logOut}>
			<LogOutIcon />
			{text.logOut}
		</DropdownMenu.Item>
	</DropdownMenu.Content>
{/snippet}

<aside class="group relative z-40 h-svh w-[60px] shrink-0" data-profile-open={isProfileMenuOpen}>
	<div data-app-rail class="absolute inset-y-0 left-0 flex w-[60px] flex-col gap-1 overflow-hidden border-r border-sidebar-border bg-sidebar py-2.5 transition-[width,box-shadow] duration-150 ease-out group-hover:w-[224px] group-hover:shadow-xl group-focus-within:w-[224px] group-focus-within:shadow-xl group-data-[profile-open=true]:w-[224px] group-data-[profile-open=true]:shadow-xl">
		<a
			href="/admin/"
			aria-label="Blueclaw"
			class="mx-2.5 mb-1 flex h-10 w-10 items-center gap-3 overflow-hidden rounded-md text-sm font-semibold text-sidebar-accent-foreground transition-[width] duration-150 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring group-hover:w-[204px] group-focus-within:w-[204px] group-data-[profile-open=true]:w-[204px]"
			data-sveltekit-preload-data="off"
			data-sveltekit-preload-code="off"
		>
			<span class="flex size-10 shrink-0 items-center justify-center rounded-md bg-background ring-1 ring-sidebar-border">
				<img src="/logo.svg" alt="" class="size-7" />
			</span>
			<span class="min-w-0 max-w-0 truncate opacity-0 transition-[max-width,opacity] duration-150 ease-out group-hover:max-w-[148px] group-hover:opacity-100 group-focus-within:max-w-[148px] group-focus-within:opacity-100 group-data-[profile-open=true]:max-w-[148px] group-data-[profile-open=true]:opacity-100">Blueclaw</span>
		</a>

		<nav class="flex min-h-0 flex-1 flex-col gap-1">
			{#each apps as item (item.href)}
				{@render railLink(item)}
			{/each}

			<div class="mx-[18px] my-1 h-px w-6 bg-sidebar-border transition-[width,margin] duration-150 ease-out group-hover:mx-4 group-hover:w-48 group-focus-within:mx-4 group-focus-within:w-48 group-data-[profile-open=true]:mx-4 group-data-[profile-open=true]:w-48" aria-hidden="true"></div>

			{#each workspace as item (item.href)}
				{@render railLink(item)}
			{/each}
		</nav>

		<DropdownMenu.Root bind:open={isProfileMenuOpen}>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<button
						{...props}
						type="button"
						aria-label={displayUserName}
						class="mx-2 flex h-11 w-11 items-center gap-3 overflow-hidden rounded-full text-left transition-[width,background-color] duration-150 ease-out hover:bg-sidebar-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring group-hover:w-[208px] group-focus-within:w-[208px] group-data-[profile-open=true]:w-[208px]"
					>
						<span class="relative flex size-11 shrink-0 items-center justify-center rounded-full bg-background shadow-sm ring-1 ring-sidebar-border">
							<PersonAvatar name={displayUserName} email={userEmail} class="size-9 rounded-full" />
							<span class="absolute bottom-0.5 right-0.5 size-2.5 rounded-full border-2 border-sidebar bg-success" aria-hidden="true"></span>
						</span>
						<span class="grid min-w-0 max-w-0 leading-tight opacity-0 transition-[max-width,opacity] duration-150 ease-out group-hover:max-w-[140px] group-hover:opacity-100 group-focus-within:max-w-[140px] group-focus-within:opacity-100 group-data-[profile-open=true]:max-w-[140px] group-data-[profile-open=true]:opacity-100">
							<span class="truncate text-sm font-medium text-sidebar-accent-foreground">{displayUserName}</span>
							<span class="truncate text-xs text-sidebar-foreground/60">{userEmail || text.activeWorkspace}</span>
						</span>
					</button>
				{/snippet}
			</DropdownMenu.Trigger>
			{@render profileMenuContent()}
		</DropdownMenu.Root>
	</div>
</aside>

<AccountAPITokenSheet bind:open={isAPITokenSheetOpen} />

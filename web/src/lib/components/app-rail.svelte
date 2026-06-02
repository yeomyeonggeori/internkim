<script lang="ts">
	import { page } from '$app/state';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
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
	import { onMount } from 'svelte';

	type RailItem = {
		href: string;
		label: string;
		icon: typeof MailIcon;
	};

	let userEmail = $state('');
	const text = createPageText(appShellText);
	let userName = $state('');
	const displayUserName = $derived(userName || text.workspace);

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
			if (!response.ok) return;
			const session = (await response.json()) as { email?: string; claimedAdminEmail?: string };
			userEmail = session.email || session.claimedAdminEmail || '';
			userName = userEmail ? userEmail.split('@')[0] : '';
		} catch {
			userEmail = '';
		}
	}
</script>

<aside class="flex h-svh w-[60px] shrink-0 flex-col items-center gap-1 border-r border-sidebar-border bg-sidebar py-2.5">
	<a
		href="/admin/"
		aria-label="Blueclaw"
		class="mb-1 flex size-10 items-center justify-center rounded-md bg-background ring-1 ring-sidebar-border"
		data-sveltekit-preload-data="off"
		data-sveltekit-preload-code="off"
	>
		<img src="/logo.svg" alt="Blueclaw" class="size-7" />
	</a>

	{#each apps as item (item.href)}
		{@const Icon = item.icon}
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<a
						{...props}
						href={item.href}
						aria-label={item.label}
						data-active={isActive(item.href)}
						data-sveltekit-preload-data="off"
						data-sveltekit-preload-code="off"
						class="relative flex size-10 items-center justify-center rounded-md text-sidebar-foreground/70 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground"
					>
						<Icon class="size-5" />
					</a>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content side="right" sideOffset={6}>{item.label}</Tooltip.Content>
		</Tooltip.Root>
	{/each}

	<div class="my-1 h-px w-6 bg-sidebar-border" aria-hidden="true"></div>

	{#each workspace as item (item.href)}
		{@const Icon = item.icon}
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<a
						{...props}
						href={item.href}
						aria-label={item.label}
						data-active={isActive(item.href)}
						data-sveltekit-preload-data="off"
						data-sveltekit-preload-code="off"
						class="flex size-10 items-center justify-center rounded-md text-sidebar-foreground/70 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground"
					>
						<Icon class="size-5" />
					</a>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content side="right" sideOffset={6}>{item.label}</Tooltip.Content>
		</Tooltip.Root>
	{/each}

	<div class="flex-1"></div>

	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					{...props}
					type="button"
					aria-label={displayUserName}
					class="relative flex size-11 items-center justify-center rounded-full bg-background shadow-sm ring-1 ring-sidebar-border transition-colors hover:bg-sidebar-accent"
				>
					<PersonAvatar name={displayUserName} email={userEmail} class="size-9 rounded-full" />
					<span class="absolute bottom-0.5 right-0.5 size-2.5 rounded-full border-2 border-sidebar bg-success" aria-hidden="true"></span>
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content side="right" align="end" sideOffset={6} class="min-w-56">
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
			<DropdownMenu.Item onclick={() => (location.href = '/flow/')}>
				<BellIcon />
				{text.activity}
			</DropdownMenu.Item>
			<DropdownMenu.Separator />
			<DropdownMenu.Item onclick={() => (location.href = '/cdn-cgi/access/logout')}>
				<LogOutIcon />
				{text.logOut}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</aside>

<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { useSidebar } from '$lib/components/ui/sidebar/index.js';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import BadgeCheckIcon from '@lucide/svelte/icons/badge-check';
	import BellIcon from '@lucide/svelte/icons/bell';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import CogIcon from '@lucide/svelte/icons/cog';
	import ListChecksIcon from '@lucide/svelte/icons/list-checks';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import MailIcon from '@lucide/svelte/icons/mail';
	import { onMount } from 'svelte';

	type NavigationItem = {
		href: string;
		label: string;
		icon: typeof MailIcon;
	};

	let { activePath }: { activePath: string } = $props();

	let userEmail = $state('');
	let userName = $state('Workspace');

	const sidebar = useSidebar();
	const appItems: NavigationItem[] = [
		{ href: '/flow/', label: 'Flow', icon: ListChecksIcon },
		{ href: '/calendar/', label: 'Calendar', icon: CalendarDaysIcon },
		{ href: '/mail/', label: 'Mail', icon: MailIcon }
	];

	const workspaceItems: NavigationItem[] = [{ href: '/admin/', label: 'Admin', icon: CogIcon }];

	function isActive(href: string) {
		return activePath === href || activePath.startsWith(href);
	}

	function classNameWith(value: unknown, extraClassName: string) {
		if (typeof value !== 'string') return extraClassName;
		return `${value} ${extraClassName}`;
	}

	onMount(loadUser);

	async function loadUser() {
		try {
			const response = await fetch('/admin/api/session', { credentials: 'include' });
			if (!response.ok) return;
			const session = (await response.json()) as { email?: string; claimedAdminEmail?: string };
			userEmail = session.email || session.claimedAdminEmail || '';
			userName = userEmail ? userEmail.split('@')[0] : 'Workspace';
		} catch {
			userEmail = '';
		}
	}
</script>

<Sidebar.Root collapsible="icon">
	<Sidebar.Header class="border-b border-sidebar-border">
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<Sidebar.MenuButton size="lg" tooltipContent="Intern Kim">
					{#snippet child({ props })}
						<a
							{...props}
							href="/admin/"
							data-sveltekit-preload-data="off"
							data-sveltekit-preload-code="off"
							class={classNameWith(props.class, 'h-12')}
						>
							<div class="flex aspect-square size-8 items-center justify-center overflow-hidden rounded-md bg-background ring-1 ring-sidebar-border">
								<img src="/logo.svg" alt="Blueclaw" class="size-7" />
							</div>
							<div class="grid min-w-0 flex-1 text-left text-sm leading-tight">
								<span class="truncate font-semibold">Blueclaw</span>
								<span class="truncate text-xs text-sidebar-foreground/60">Intern Kim</span>
							</div>
						</a>
					{/snippet}
				</Sidebar.MenuButton>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Header>

	<Sidebar.Content>
		<Sidebar.Group>
			<Sidebar.GroupLabel>Apps</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<Sidebar.Menu>
					{#each appItems as item (item.href)}
						{@const Icon = item.icon}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton isActive={isActive(item.href)} tooltipContent={item.label}>
								{#snippet child({ props })}
									<a {...props} href={item.href} data-sveltekit-preload-data="off" data-sveltekit-preload-code="off">
										<Icon />
										<span>{item.label}</span>
									</a>
								{/snippet}
							</Sidebar.MenuButton>
						</Sidebar.MenuItem>
					{/each}
				</Sidebar.Menu>
			</Sidebar.GroupContent>
		</Sidebar.Group>

		<Sidebar.Group>
			<Sidebar.GroupLabel>Workspace</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<Sidebar.Menu>
					{#each workspaceItems as item (item.href)}
						{@const Icon = item.icon}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton isActive={isActive(item.href)} tooltipContent={item.label}>
								{#snippet child({ props })}
									<a {...props} href={item.href} data-sveltekit-preload-data="off" data-sveltekit-preload-code="off">
										<Icon />
										<span>{item.label}</span>
									</a>
								{/snippet}
							</Sidebar.MenuButton>
						</Sidebar.MenuItem>
					{/each}
				</Sidebar.Menu>
			</Sidebar.GroupContent>
		</Sidebar.Group>
	</Sidebar.Content>

	<Sidebar.Footer class="border-t border-sidebar-border">
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Sidebar.MenuButton
								size="lg"
								class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
								{...props}
							>
								<PersonAvatar name={userName} email={userEmail} class="size-8 rounded-lg" />
								<div class="grid flex-1 text-start text-sm leading-tight">
									<span class="truncate font-medium">{userName}</span>
									<span class="truncate text-xs">{userEmail || 'Active workspace'}</span>
								</div>
								<ChevronsUpDownIcon class="ms-auto size-4" />
							</Sidebar.MenuButton>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content
						class="w-(--bits-dropdown-menu-anchor-width) min-w-56 rounded-lg"
						side={sidebar.isMobile ? 'bottom' : 'right'}
						align="end"
						sideOffset={4}
					>
						<DropdownMenu.Label class="p-0 font-normal">
							<div class="flex items-center gap-2 px-1 py-1.5 text-start text-sm">
								<PersonAvatar name={userName} email={userEmail} class="size-8 rounded-lg" />
								<div class="grid flex-1 text-start text-sm leading-tight">
									<span class="truncate font-medium">{userName}</span>
									<span class="truncate text-xs">{userEmail || 'Active workspace'}</span>
								</div>
							</div>
						</DropdownMenu.Label>
						<DropdownMenu.Separator />
						<DropdownMenu.Group>
							<DropdownMenu.Item onclick={() => (location.href = '/admin/')}>
								<BadgeCheckIcon />
								Account
							</DropdownMenu.Item>
							<DropdownMenu.Item onclick={() => (location.href = '/flow/')}>
								<BellIcon />
								Activity
							</DropdownMenu.Item>
						</DropdownMenu.Group>
						<DropdownMenu.Separator />
						<DropdownMenu.Item onclick={() => (location.href = '/cdn-cgi/access/logout')}>
							<LogOutIcon />
							Log out
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Footer>
</Sidebar.Root>

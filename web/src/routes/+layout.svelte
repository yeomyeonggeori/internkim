<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import ChannelSheet from '$lib/components/channel/channel-sheet.svelte';
	import AppCommandPalette from '$lib/components/app-command-palette.svelte';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import AppRail from '$lib/components/app-rail.svelte';
	import WebAuthGate from '$lib/components/web-auth-gate.svelte';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { LightSwitch } from '$lib/components/ui/light-switch';
	import { Button } from '$lib/components/ui/button';
	import { Kbd } from '$lib/components/ui/kbd';
	import { LanguageSwitcher } from '$lib/components/ui/language-switcher';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { Toaster } from '$lib/components/ui/sonner';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { currentLocale, initializeLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { isEmbeddedFrame } from '$lib/embedded';
	import { isAppShortcutMessage } from '$lib/app-shortcut-message';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { ModeWatcher } from 'mode-watcher';
	import { onMount } from 'svelte';

	let { children } = $props();
	const text = createPageText(appShellText);
	let isCommandPaletteOpen = $state(false);
	let isAppSidebarOpen = $state(false);

	onMount(initializeLocale);

	function selectLocale(code: string) {
		if (code === 'ko' || code === 'en') setLocale(code);
	}

	function usesAppShell(pathname: string) {
		if (pathname === '/calendar/embed' || pathname.startsWith('/calendar/embed/')) return false;
		return ['/settings/', '/poc-admin/', '/messenger/', '/flow/', '/memory/', '/calendar/', '/mail/', '/attendance/', '/organization/', '/files/', '/tasks/', '/assistant/'].some(
			(prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix)
		);
	}

	function currentApp(pathname: string) {
		if (pathname.startsWith('/settings')) return text.settings;
		if (pathname.startsWith('/poc-admin')) return text.pocAdmin;
		if (pathname.startsWith('/tasks')) return text.tasks;
		if (pathname.startsWith('/memory')) return text.memory;
		if (pathname.startsWith('/calendar')) return text.calendar;
		if (pathname.startsWith('/mail')) return text.mail;
		if (pathname.startsWith('/attendance')) return text.attendance;
		if (pathname.startsWith('/organization')) return text.organization;
		if (pathname.startsWith('/files')) return text.files;
		if (pathname.startsWith('/assistant')) return text.assistant;
		if (pathname.startsWith('/messenger')) return text.messenger;
		return text.flow;
	}

	function currentAppPath(pathname: string) {
		return `/${pathname.split('/')[1] ?? ''}/`;
	}

	function usesWebAuthGate(pathname: string) {
		return ['/messenger/', '/flow/', '/memory/', '/calendar/', '/mail/', '/attendance/', '/organization/', '/files/', '/tasks/', '/assistant/', '/poc-admin/', '/settings/'].some(
			(prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix)
		);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (isCommandPaletteOpen) return;
		if (isPlainShortcut(event, 'Slash')) {
			event.preventDefault();
			isCommandPaletteOpen = true;
			return;
		}
		if (!isPlainShortcut(event, 'KeyR')) return;
		event.preventDefault();
		pageActions.refresh();
	}

	function handleFrameShortcut(event: MessageEvent<unknown>) {
		if (event.origin !== window.location.origin || !isAppShortcutMessage(event.data)) return;
		if (event.data.code === 'Slash') {
			isCommandPaletteOpen = true;
			return;
		}
		pageActions.refresh();
	}

	function currentReturnPath() {
		return page.url.pathname + page.url.search;
	}
</script>

<svelte:head>
	<link rel="icon" href="/logo.svg" />
</svelte:head>

<svelte:window onkeydown={handleKeydown} onmessage={handleFrameShortcut} />

<ModeWatcher />
<Toaster position="bottom-center" visibleToasts={3} containerAriaLabel={text.notifications} />

{#if usesAppShell(page.url.pathname)}
	<Tooltip.Provider delayDuration={120}>
		<Sidebar.Provider bind:open={isAppSidebarOpen} class="flex h-svh w-full bg-background text-foreground">
			{#if !isEmbeddedFrame()}
				<AppRail />
			{/if}
			<div class="flex min-w-0 flex-1 flex-col">
				{#if !isEmbeddedFrame()}
					<header data-app-chrome class="internkim-app-header">
						<Tooltip.Root>
							<Tooltip.Trigger>
								{#snippet child({ props })}
									<Sidebar.Trigger {...props} class="-ml-1 max-sm:hidden" />
								{/snippet}
							</Tooltip.Trigger>
							<Tooltip.Content side="bottom">
								{text.sidebar}
								<Kbd>,</Kbd>
							</Tooltip.Content>
						</Tooltip.Root>
						<Separator orientation="vertical" class="mr-2 !h-4 max-sm:hidden" />
						<div class="internkim-app-crumbs flex-1">
							<Breadcrumb.Root>
								<Breadcrumb.List>
									<Breadcrumb.Item>
										{#if breadcrumbMeta.value}
											<Breadcrumb.Link
												href={currentAppPath(page.url.pathname)}
												data-sveltekit-preload-data="off"
												onclick={breadcrumbMeta.clear}
											>
												{currentApp(page.url.pathname)}
											</Breadcrumb.Link>
										{:else}
											<Breadcrumb.Page>{currentApp(page.url.pathname)}</Breadcrumb.Page>
										{/if}
									</Breadcrumb.Item>
									{#if breadcrumbMeta.value}
										<Breadcrumb.Separator />
										<Breadcrumb.Item>
											<Breadcrumb.Page>{breadcrumbMeta.value}</Breadcrumb.Page>
										</Breadcrumb.Item>
									{/if}
								</Breadcrumb.List>
							</Breadcrumb.Root>
						</div>
						<div class="flex items-center gap-2">
							{#if page.url.pathname.startsWith('/messenger')}
								<ChannelSheet />
							{/if}
							<Button
								variant="outline"
								size="sm"
								class="text-muted-foreground hidden w-48 justify-start gap-2 font-normal sm:flex"
								onclick={() => (isCommandPaletteOpen = true)}
							>
								<SearchIcon class="size-4" />
								{text.search}
								<Kbd class="ml-auto">/</Kbd>
							</Button>
							<Tooltip.Root>
									<Tooltip.Trigger>
										{#snippet child({ props })}
											<Button
												{...props}
												variant="ghost"
												size="icon-sm"
												aria-label={text.refresh}
												onclick={pageActions.refresh}
											>
												<RefreshCwIcon class={pageActions.isRefreshing ? 'animate-spin' : ''} />
											</Button>
										{/snippet}
									</Tooltip.Trigger>
									<Tooltip.Content side="bottom">
										{text.refresh}
										<Kbd>r</Kbd>
									</Tooltip.Content>
							</Tooltip.Root>
							<LanguageSwitcher
								variant="ghost"
								languages={localeOptions.map((option) => ({ code: option.value, label: option.label }))}
								value={currentLocale.value}
								ariaLabel={text.changeLanguage}
								onChange={selectLocale}
							/>
							<LightSwitch variant="ghost" />
						</div>
					</header>
				{/if}
				<div data-app-shell-scroll class="flex min-h-0 flex-1 overflow-y-auto max-sm:pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:pb-0">
					{#if usesWebAuthGate(page.url.pathname)}
						<WebAuthGate returnPath={currentReturnPath()}>
							{@render children()}
						</WebAuthGate>
					{:else}
						{@render children()}
					{/if}
				</div>
			</div>
		</Sidebar.Provider>
		<AppCommandPalette bind:open={isCommandPaletteOpen} />
	</Tooltip.Provider>
{:else}
	{@render children()}
{/if}

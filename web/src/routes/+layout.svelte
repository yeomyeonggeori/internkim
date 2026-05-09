<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import { ThemeSelector } from '$lib/components/ui/theme-selector';
	import LanguageSelector from '$lib/i18n/language-selector.svelte';
	import { initializeLocale } from '$lib/i18n/locale.svelte';
	import { page } from '$app/state';
	import { ModeWatcher } from 'mode-watcher';
	import { onMount } from 'svelte';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { Separator } from '$lib/components/ui/separator';

	let { children } = $props();

	onMount(initializeLocale);

	function usesAppShell(pathname: string) {
		if (pathname === '/calendar/embed' || pathname.startsWith('/calendar/embed/')) return false;
		return ['/admin/', '/flow/', '/calendar/', '/mail/'].some((prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix));
	}

	function currentApp(pathname: string) {
		if (pathname.startsWith('/admin')) return 'Admin';
		if (pathname.startsWith('/calendar')) return 'Calendar';
		if (pathname.startsWith('/mail')) return 'Mail';
		return 'Flow';
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<ModeWatcher />

{#if usesAppShell(page.url.pathname)}
	<Sidebar.Provider class="internkim-app-shell">
		<AppSidebar activePath={page.url.pathname} />
		<Sidebar.Inset class="internkim-app-inset">
			<header class="internkim-app-header">
				<Sidebar.Trigger />
				<Separator orientation="vertical" class="internkim-app-header-separator" />
				<div class="internkim-app-crumbs flex-1">
					<Breadcrumb.Root>
						<Breadcrumb.List>
							<Breadcrumb.Item class="hidden md:block">
								<Breadcrumb.Link href="/admin/" data-sveltekit-preload-data="off" data-sveltekit-preload-code="off">Blueclaw</Breadcrumb.Link>
							</Breadcrumb.Item>
							<Breadcrumb.Separator class="hidden md:block" />
							<Breadcrumb.Item>
								<Breadcrumb.Page>{currentApp(page.url.pathname)}</Breadcrumb.Page>
							</Breadcrumb.Item>
						</Breadcrumb.List>
					</Breadcrumb.Root>
				</div>
				<div class="flex items-center gap-2">
					<LanguageSelector />
					<ThemeSelector variant="ghost" />
				</div>
			</header>
			<div class="internkim-app-content">
				{@render children()}
			</div>
		</Sidebar.Inset>
	</Sidebar.Provider>
{:else}
	{@render children()}
{/if}

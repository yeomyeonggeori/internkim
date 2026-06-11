<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import AppRail from '$lib/components/app-rail.svelte';
	import WebAuthGate from '$lib/components/web-auth-gate.svelte';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { ThemeSelector } from '$lib/components/ui/theme-selector';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import LanguageSelector from '$lib/i18n/language-selector.svelte';
	import { initializeLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { ModeWatcher } from 'mode-watcher';
	import { onMount } from 'svelte';

	let { children } = $props();
	const text = createPageText(appShellText);

	onMount(initializeLocale);

	function usesAppShell(pathname: string) {
		if (pathname === '/calendar/embed' || pathname.startsWith('/calendar/embed/')) return false;
		return ['/admin/', '/flow/', '/memory/', '/calendar/', '/mail/', '/attendance/', '/tasks/'].some(
			(prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix)
		);
	}

	function currentApp(pathname: string) {
		if (pathname.startsWith('/admin')) return text.admin;
		if (pathname.startsWith('/tasks')) return text.tasks;
		if (pathname.startsWith('/memory')) return text.memory;
		if (pathname.startsWith('/calendar')) return text.calendar;
		if (pathname.startsWith('/mail')) return text.mail;
		if (pathname.startsWith('/attendance')) return text.attendance;
		return text.flow;
	}

	function usesWebAuthGate(pathname: string) {
		return ['/flow/', '/memory/', '/calendar/', '/mail/', '/attendance/'].some(
			(prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix)
		);
	}

	function currentReturnPath() {
		return page.url.pathname + page.url.search;
	}
</script>

<svelte:head>
	<link rel="icon" href="/logo.svg" />
</svelte:head>

<ModeWatcher />

{#if usesAppShell(page.url.pathname)}
	<Tooltip.Provider delayDuration={120}>
		<div class="flex h-svh w-full bg-background text-foreground">
			<AppRail />
			<div class="flex min-w-0 flex-1 flex-col">
				<header class="internkim-app-header">
					<div class="internkim-app-crumbs flex-1">
						<Breadcrumb.Root>
							<Breadcrumb.List>
								<Breadcrumb.Item class="hidden md:block">
									<Breadcrumb.Link href="/admin/" data-sveltekit-preload-data="off" data-sveltekit-preload-code="off">Blueclaw</Breadcrumb.Link>
								</Breadcrumb.Item>
								<Breadcrumb.Separator class="hidden md:block" />
								<Breadcrumb.Item>
									<Breadcrumb.Page>
										<span>{currentApp(page.url.pathname)}</span>
										{#if breadcrumbMeta.value}
											<span class="ml-1.5 font-normal text-muted-foreground">· {breadcrumbMeta.value}</span>
										{/if}
									</Breadcrumb.Page>
								</Breadcrumb.Item>
							</Breadcrumb.List>
						</Breadcrumb.Root>
					</div>
					<div class="flex items-center gap-2">
						<LanguageSelector />
						<ThemeSelector variant="ghost" />
					</div>
				</header>
				<div class="flex min-h-0 flex-1 overflow-y-auto">
					{#if usesWebAuthGate(page.url.pathname)}
						<WebAuthGate returnPath={currentReturnPath()}>
							{@render children()}
						</WebAuthGate>
					{:else}
						{@render children()}
					{/if}
				</div>
			</div>
		</div>
	</Tooltip.Provider>
{:else}
	{@render children()}
{/if}

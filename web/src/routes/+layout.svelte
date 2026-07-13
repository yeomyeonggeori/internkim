<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import AppRail from '$lib/components/app-rail.svelte';
	import WebAuthGate from '$lib/components/web-auth-gate.svelte';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { LightSwitch } from '$lib/components/ui/light-switch';
	import { LanguageSwitcher } from '$lib/components/ui/language-switcher';
	import { Toaster } from '$lib/components/ui/sonner';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { currentLocale, initializeLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { isEmbeddedFrame } from '$lib/embedded';
	import { ModeWatcher } from 'mode-watcher';
	import { onMount } from 'svelte';

	let { children } = $props();
	const text = createPageText(appShellText);

	onMount(initializeLocale);

	function selectLocale(code: string) {
		if (code === 'ko' || code === 'en') setLocale(code);
	}

	function usesAppShell(pathname: string) {
		if (pathname === '/calendar/embed' || pathname.startsWith('/calendar/embed/')) return false;
		return ['/admin/', '/poc-admin/', '/flow/', '/memory/', '/calendar/', '/mail/', '/attendance/', '/orgchart/', '/files/', '/tasks/'].some(
			(prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix)
		);
	}

	function currentApp(pathname: string) {
		if (pathname.startsWith('/admin')) return text.admin;
		if (pathname.startsWith('/poc-admin')) return text.pocAdmin;
		if (pathname.startsWith('/tasks')) return text.tasks;
		if (pathname.startsWith('/memory')) return text.memory;
		if (pathname.startsWith('/calendar')) return text.calendar;
		if (pathname.startsWith('/mail')) return text.mail;
		if (pathname.startsWith('/attendance')) return text.attendance;
		if (pathname.startsWith('/orgchart')) return text.orgchart;
		if (pathname.startsWith('/files')) return text.files;
		return text.flow;
	}

	function usesWebAuthGate(pathname: string) {
		return ['/flow/', '/memory/', '/calendar/', '/mail/', '/attendance/', '/orgchart/', '/files/', '/tasks/', '/poc-admin/'].some(
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
<Toaster position="bottom-center" visibleToasts={3} containerAriaLabel={text.notifications} />

{#if usesAppShell(page.url.pathname)}
	<Tooltip.Provider delayDuration={120}>
		<div class="flex h-svh w-full bg-background text-foreground">
			{#if !isEmbeddedFrame()}
				<AppRail />
			{/if}
			<div class="flex min-w-0 flex-1 flex-col">
				{#if !isEmbeddedFrame()}
					<header data-app-chrome class="internkim-app-header">
						<a
							href="/admin/"
							aria-label="Blueclaw"
							class="internkim-app-mobile-brand"
							data-sveltekit-preload-data="off"
							data-sveltekit-preload-code="off"
						>
							<img src="/logo.svg" alt="" />
						</a>
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
				<div data-app-shell-scroll class="flex min-h-0 flex-1 overflow-y-auto max-md:pb-[calc(1.25rem+env(safe-area-inset-bottom))] md:pb-0">
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

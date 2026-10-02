<script lang="ts">
	import '../app.css';
	import { goto, invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import AppCommandPalette from '$lib/components/app-command-palette.svelte';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import AppRail from '$lib/components/app-rail.svelte';
	import EffectErrorBoundary from '$lib/components/effect-error-boundary.svelte';
	import { appSectionPathOf, usesAppShell, usesWebAuthGate } from '$lib/app-shell';
	import { routePathOf } from '$lib/company-path';
	import BuzzIdentityGate from '$lib/components/buzz/buzz-identity-gate.svelte';
	import WebAuthGate from '$lib/components/web-auth-gate.svelte';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { LightSwitch } from '$lib/components/ui/light-switch';
	import { Button } from '$lib/components/ui/button';
	import { Kbd } from '$lib/components/ui/kbd';
	import { LanguageSwitcher } from '$lib/components/ui/language-switcher';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { Toaster } from '$lib/components/ui/sonner';
	import { toast } from 'svelte-sonner';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { currentLocale, initializeLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { isEmbeddedFrame } from '$lib/embedded';
	import { isAppShortcutMessage } from '$lib/app-shortcut-message';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { goWhereNotificationsPoint } from '$lib/notifications/opened-notification';
	import { webAuthSessionDependency } from '$lib/web-auth-session';
	import { forgetSignedInAccount } from '$lib/signed-in-account-memo';
	import { keepShellStatusBarOnPageTheme } from '$lib/native-shell/page-theme';
	import { goWhereNativeNotificationsPoint, keepNativeDeviceClaimed } from '$lib/notifications/native-device';
	import { askToBeReachedOnce } from '$lib/notifications/ask-once';
	import { keepWidgetSupplied } from '$lib/widget/attendance-widget-supply';
	import { keepActivityTokensClaimed } from '$lib/widget/attendance-activity-tokens';
	import { catchTheLockScreenUp } from '$lib/attendance/lock-screen-catch-up';
	import { setPersonNameCompanyLocale } from '$lib/person-name.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { ModeWatcher } from 'mode-watcher';
	import { onMount, untrack } from 'svelte';
	import { keepMemberPicture } from '$lib/profile/keep-member-picture';
	import { memberPresence } from '$lib/messenger/member-presence.svelte';


	let { children, data } = $props();
	const text = createPageText(appShellText);
	let isCommandPaletteOpen = $state(false);
	let isAppSidebarOpen = $state(false);
	let attendanceSessionKey = '';
	let widgetSuppliedEmail = '';
	$effect(() => setPersonNameCompanyLocale(data.companyLocale ?? ''));
	$effect(() => {
		const sessionKey = `${data.session?.authenticated ?? false}:${data.session?.email ?? ''}`;
		untrack(() => {
			const hasSessionChanged = sessionKey !== attendanceSessionKey;
			attendanceSessionKey = sessionKey;
			if (hasSessionChanged) myAttendanceToday.clear();
		});
	});
	$effect(() => {
		if (!data.session?.authenticated) return;
		return memberPresence.keepMineShared();
	});
	$effect(() => {
		if (!data.session?.authenticated) return;
		askToBeReachedOnce().then(
			(answer) => {
				if (answer === 'refused') toast.info(text.notifyLater);
			},
			(failure: unknown) => console.warn('this device was not asked about notifications', failure)
		);
	});
	$effect(() => {
		const signedInEmail = data.session?.authenticated ? data.session.email : '';
		untrack(() => {
			if (!signedInEmail || signedInEmail === widgetSuppliedEmail) return;
			widgetSuppliedEmail = signedInEmail;
			keepWidgetSupplied().catch((failure: unknown) =>
				console.warn('the widget holds no key of its own', failure)
			);
			keepActivityTokensClaimed().catch((failure: unknown) =>
				console.warn('the lock screen is not following attendance', failure)
			);
			catchTheLockScreenUp();
		});
	});
	onMount(() => {
		initializeLocale();
		if (data.session?.authenticated) {
			keepMemberPicture(data.session.email).catch((failure: unknown) =>
				console.warn('the member picture was not kept', failure)
			);
		}
		let stopFollowingPageTheme = () => {};
		keepShellStatusBarOnPageTheme().then(
			(stop) => {
				stopFollowingPageTheme = stop;
			},
			(failure: unknown) => console.warn('the shell status bar is not following the page theme', failure)
		);

		const revalidateSession = () => {
			if (document.visibilityState !== 'visible') return;
			forgetSignedInAccount();
			void invalidate(webAuthSessionDependency);
		};
		window.addEventListener('focus', revalidateSession);
		document.addEventListener('visibilitychange', revalidateSession);
		const stopFollowingNotifications = goWhereNotificationsPoint((path) => void goto(path));
		let stopFollowingNativeNotifications = () => {};
		goWhereNativeNotificationsPoint((path) => void goto(path)).then(
			(stop) => {
				stopFollowingNativeNotifications = stop;
			},
			(failure: unknown) => console.warn('the shell is not following notification taps', failure)
		);
		keepNativeDeviceClaimed().catch((failure: unknown) =>
			console.warn('this device is not claimed for push', failure)
		);
		return () => {
			stopFollowingPageTheme();
			stopFollowingNativeNotifications();
			stopFollowingNotifications();
			window.removeEventListener('focus', revalidateSession);
			document.removeEventListener('visibilitychange', revalidateSession);
		};
	});

	function selectLocale(code: string) {
		if (code === 'ko' || code === 'en') setLocale(code);
	}

	function currentApp(routePath: string) {
		if (routePath.startsWith('/auth/claim')) return text.claimTitle;
		if (routePath.startsWith('/settings')) return text.settings;
		if (routePath.startsWith('/runs')) return text.tasks;
		if (routePath.startsWith('/memory')) return text.memory;
		if (routePath.startsWith('/calendar')) return text.calendar;
		if (routePath.startsWith('/mail')) return text.mail;
		if (routePath.startsWith('/attendance')) return text.attendance;
		if (routePath.startsWith('/crm')) return text.crm;
		if (routePath.startsWith('/organization')) return text.organization;
		if (routePath.startsWith('/files')) return text.files;
		if (routePath.startsWith('/data-room')) return text.dataRoom;
		if (routePath.startsWith('/assistant')) return text.assistant;
		if (routePath.startsWith('/messenger')) return text.messenger;
		return text.task;
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

{#snippet contained()}
	<EffectErrorBoundary region="page">
		{@render children()}
	</EffectErrorBoundary>
{/snippet}

<svelte:window onkeydown={handleKeydown} onmessage={handleFrameShortcut} />

<ModeWatcher />
<Toaster position="bottom-center" visibleToasts={3} containerAriaLabel={text.notifications} />
<BuzzIdentityGate />

{#if usesAppShell(page.url.pathname)}
	<Tooltip.Provider delayDuration={120}>
		<Sidebar.Provider bind:open={isAppSidebarOpen} class="flex h-[min(100svh,100%)] min-h-0 w-full bg-background text-foreground">
			{#if !isEmbeddedFrame()}
				<EffectErrorBoundary region="app rail">
					<AppRail session={data.session} />
				</EffectErrorBoundary>
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
												href={appSectionPathOf(page.url.pathname)}
												data-sveltekit-preload-data="off"
												onclick={breadcrumbMeta.clear}
											>
												{currentApp(routePathOf(page.url.pathname))}
											</Breadcrumb.Link>
										{:else}
											<Breadcrumb.Page>{currentApp(routePathOf(page.url.pathname))}</Breadcrumb.Page>
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
				<div data-app-shell-scroll class="flex min-h-0 flex-1 overflow-y-auto max-sm:pb-[calc(var(--app-mobile-nav-bottom)+var(--app-mobile-nav-height)+0.5rem)] sm:pb-0">
					{#if usesWebAuthGate(page.url.pathname)}
						<WebAuthGate session={data.session} returnPath={currentReturnPath()}>
							{@render contained()}
						</WebAuthGate>
					{:else}
						{@render contained()}
					{/if}
				</div>
			</div>
		</Sidebar.Provider>
		<AppCommandPalette bind:open={isCommandPaletteOpen} />
	</Tooltip.Provider>
{:else}
	{@render contained()}
{/if}

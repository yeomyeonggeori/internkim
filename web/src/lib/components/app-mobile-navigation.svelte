<script lang="ts" module>
	import MailIcon from '@lucide/svelte/icons/mail';

	export type AppMobileNavigationItem = {
		href: string;
		label: string;
		icon: typeof MailIcon;
	};
</script>

<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import PowerOffIcon from '@lucide/svelte/icons/power-off';
	import MoreHorizontalIcon from '@lucide/svelte/icons/more-horizontal';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { tick } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { LightSwitch } from '$lib/components/ui/light-switch';
	import { LanguageSwitcher } from '$lib/components/ui/language-switcher';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { currentLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	type AppMobileNavigationText = {
		activeWorkspace: string;
		apps: string;
		close: string;
		logOut: string;
		more: string;
		moreDescription: string;
	};

	let {
		displayUserName,
		isActive,
		logOut,
		moreItems,
		primaryItems,
		onSearch,
		text,
		userEmail,
		userImage,
		userMemberID
	}: {
		displayUserName: string;
		isActive: (href: string) => boolean;
		logOut: () => void | Promise<void>;
		moreItems: AppMobileNavigationItem[];
		primaryItems: AppMobileNavigationItem[];
		onSearch?: () => void;
		text: AppMobileNavigationText;
		userEmail: string;
		userImage?: string;
		userMemberID?: string;
	} = $props();

	let isMoreSheetOpen = $state(false);
	const shellText = createPageText(appShellText);
	const isMobile = new IsMobile();
	const isMoreActive = $derived(moreItems.some((item) => isActive(item.href)));

	$effect(() => {
		if (isMobile.current) return;
		isMoreSheetOpen = false;
	});

	function closeMoreSheet() {
		isMoreSheetOpen = false;
	}

	function selectLocale(code: string) {
		if (code === 'ko' || code === 'en') setLocale(code);
	}

	function refreshFromMobile() {
		closeMoreSheet();
		void pageActions.refresh();
	}

	async function searchFromMobile() {
		closeMoreSheet();
		await tick();
		onSearch?.();
	}

	async function logOutFromMobile() {
		closeMoreSheet();
		await logOut();
	}
</script>

{#snippet mobileNavigationLink(item: AppMobileNavigationItem)}
	{@const Icon = item.icon}
	<a
		href={item.href}
		aria-label={item.label}
		aria-current={isActive(item.href) ? 'page' : undefined}
		data-active={isActive(item.href)}
		data-sveltekit-preload-data="hover"
		class="flex min-h-11 min-w-0 flex-col items-center justify-center gap-1 rounded-md px-1 py-1 text-[11px] font-medium leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring data-[active=true]:text-foreground data-[active=true]:font-semibold"
	>
		<Icon class="size-5 shrink-0" />
		<span class="max-w-full truncate">{item.label}</span>
	</a>
{/snippet}

{#snippet moreSheetNavigationLink(item: AppMobileNavigationItem)}
	{@const Icon = item.icon}
	<a
		href={item.href}
		data-active={isActive(item.href)}
		data-sveltekit-preload-data="hover"
		onclick={closeMoreSheet}
		class="flex min-h-11 items-center gap-3 rounded-md px-3 text-sm font-medium text-sidebar-foreground/80 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground"
	>
		<Icon class="size-5 shrink-0" />
		<span class="min-w-0 truncate">{item.label}</span>
	</a>
{/snippet}

{#snippet mobileAccountActions()}
	<div class="rounded-md border border-sidebar-border bg-background p-2">
		<div class="flex items-center gap-3 px-1 py-1.5">
			<PersonAvatar name={displayUserName} email={userEmail} memberID={userMemberID ?? ''} image={userImage ?? ''} class="size-9 rounded-lg" />
			<div class="grid min-w-0 flex-1 text-sm leading-tight">
				<span class="truncate font-medium">{displayUserName}</span>
				<span class="truncate text-xs text-muted-foreground">{userEmail || text.activeWorkspace}</span>
			</div>
		</div>
		<div class="mt-2 grid gap-1">
			<button
				type="button"
				onclick={logOutFromMobile}
				class="flex min-h-11 items-center gap-3 rounded-md px-2 text-left text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
			>
				<PowerOffIcon class="size-4" />
				<span>{text.logOut}</span>
			</button>
		</div>
	</div>
{/snippet}

<Sheet.Root bind:open={isMoreSheetOpen}>
	<Sheet.Content side="right" class="w-[min(20rem,calc(100vw-1.5rem))] gap-0 bg-sidebar p-0 text-sidebar-foreground" showCloseButton={true} closeLabel={text.close}>
		<Sheet.Header class="min-h-16 justify-center border-b border-sidebar-border px-4 py-3">
			<Sheet.Title>{text.more}</Sheet.Title>
			<Sheet.Description class="sr-only">{text.moreDescription}</Sheet.Description>
		</Sheet.Header>
		<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-3">
			{#if onSearch}
				<Button variant="outline" class="justify-start" onclick={() => void searchFromMobile()}>
					<SearchIcon data-icon="inline-start" />{shellText.search}
				</Button>
			{/if}
			<nav class="grid gap-1">
				{#each moreItems as item (item.href)}
					{@render moreSheetNavigationLink(item)}
				{/each}
			</nav>
			<div class="h-px bg-sidebar-border" aria-hidden="true"></div>
			<div class="flex items-center gap-2">
				<Button variant="ghost" size="icon" aria-label={shellText.refresh} onclick={refreshFromMobile}>
					<RefreshCwIcon class={pageActions.isRefreshing ? 'animate-spin' : ''} />
				</Button>
				<LanguageSwitcher variant="ghost" languages={localeOptions.map((option) => ({ code: option.value, label: option.label }))} value={currentLocale.value} ariaLabel={shellText.changeLanguage} onChange={selectLocale} />
				<LightSwitch variant="ghost" />
			</div>
			{@render mobileAccountActions()}
		</div>
	</Sheet.Content>
</Sheet.Root>

<nav
	data-app-chrome
	class="internkim-app-mobile-navigation grid grid-cols-5 border-t border-sidebar-border bg-background sm:hidden"
	aria-label={text.apps}
>
	{#each primaryItems as item (item.href)}
		{@render mobileNavigationLink(item)}
	{/each}
	<button
		type="button"
		aria-label={text.more}
		aria-expanded={isMoreSheetOpen}
		data-active={isMoreActive || isMoreSheetOpen}
		onclick={() => (isMoreSheetOpen = true)}
		class="flex min-h-11 min-w-0 flex-col items-center justify-center gap-1 rounded-md px-1 py-1 text-[11px] font-medium leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring data-[active=true]:text-foreground data-[active=true]:font-semibold"
	>
		<MoreHorizontalIcon class="size-5 shrink-0" />
		<span class="max-w-full truncate">{text.more}</span>
	</button>
</nav>

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
	import { floatingAction } from '$lib/stores/floating-action.svelte';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import PowerOffIcon from '@lucide/svelte/icons/power-off';
	import MoreHorizontalIcon from '@lucide/svelte/icons/more-horizontal';

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
		text,
		userEmail,
		userImage
	}: {
		displayUserName: string;
		isActive: (href: string) => boolean;
		logOut: () => void | Promise<void>;
		moreItems: AppMobileNavigationItem[];
		primaryItems: AppMobileNavigationItem[];
		text: AppMobileNavigationText;
		userEmail: string;
		userImage?: string;
	} = $props();

	let isMoreSheetOpen = $state(false);
	const isMobile = new IsMobile();
	const isMoreActive = $derived(moreItems.some((item) => isActive(item.href)));

	$effect(() => {
		if (isMobile.current) return;
		isMoreSheetOpen = false;
	});

	function closeMoreSheet() {
		isMoreSheetOpen = false;
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
		data-active={isActive(item.href)}
		data-sveltekit-preload-data="off"
		data-sveltekit-preload-code="viewport"
		class="flex min-w-0 flex-col items-center justify-center gap-1 rounded-full px-1 py-1 text-[10.5px] font-semibold leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring data-[active=true]:bg-foreground data-[active=true]:text-background"
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
		data-sveltekit-preload-data="off"
		data-sveltekit-preload-code="viewport"
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
			<PersonAvatar name={displayUserName} email={userEmail} image={userImage ?? ''} class="size-9 rounded-lg" />
			<div class="grid min-w-0 flex-1 text-sm leading-tight">
				<span class="truncate font-medium">{displayUserName}</span>
				<span class="truncate text-xs text-muted-foreground">{userEmail || text.activeWorkspace}</span>
			</div>
		</div>
		<div class="mt-2 grid gap-1">
			<button
				type="button"
				onclick={logOutFromMobile}
				class="flex min-h-10 items-center gap-3 rounded-md px-2 text-left text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
			>
				<PowerOffIcon class="size-4" />
				<span>{text.logOut}</span>
			</button>
		</div>
	</div>
{/snippet}

<Sheet.Root bind:open={isMoreSheetOpen}>
	<Sheet.Content side="right" class="w-[min(20rem,calc(100vw-1.5rem))] gap-0 bg-sidebar p-0 text-sidebar-foreground" showCloseButton={true} closeLabel={text.close}>
		<Sheet.Header class="border-b border-sidebar-border px-4 py-3">
			<Sheet.Title>{text.more}</Sheet.Title>
			<Sheet.Description class="sr-only">{text.moreDescription}</Sheet.Description>
		</Sheet.Header>
		<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-3">
			<nav class="grid gap-1">
				{#each moreItems as item (item.href)}
					{@render moreSheetNavigationLink(item)}
				{/each}
			</nav>
			<div class="h-px bg-sidebar-border" aria-hidden="true"></div>
			{@render mobileAccountActions()}
		</div>
	</Sheet.Content>
</Sheet.Root>

<nav
	data-app-chrome
	class="internkim-app-mobile-navigation grid grid-cols-5 gap-1 rounded-full border border-sidebar-border/70 bg-background/[0.82] shadow-[0_18px_45px_rgb(15_23_42_/_0.16)] backdrop-blur-md supports-backdrop-filter:bg-background/[0.78] sm:hidden"
	data-has-floating-action={floatingAction.isPresent}
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
		class="flex min-w-0 flex-col items-center justify-center gap-1 rounded-full px-1 py-1 text-[10.5px] font-semibold leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring data-[active=true]:bg-foreground data-[active=true]:text-background"
	>
		<MoreHorizontalIcon class="size-5 shrink-0" />
		<span class="max-w-full truncate">{text.more}</span>
	</button>
</nav>

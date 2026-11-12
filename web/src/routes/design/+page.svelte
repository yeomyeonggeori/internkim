<script lang="ts">
	import { onMount, tick, type Component } from 'svelte';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button/index.js';
	import LanguageSwitcher from '$lib/components/ui/language-switcher/language-switcher.svelte';
	import LightSwitch from '$lib/components/ui/light-switch/light-switch.svelte';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as ToggleGroup from '$lib/components/ui/toggle-group/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { cn } from '$lib/utils';
	import ColumnsIcon from '@lucide/svelte/icons/columns-2';
	import FileIcon from '@lucide/svelte/icons/file';
	import FrameIcon from '@lucide/svelte/icons/frame';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import MoonIcon from '@lucide/svelte/icons/moon';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SunIcon from '@lucide/svelte/icons/sun';
	import ActionFrames from './action-frames.svelte';
	import AppFrames from './app-frames.svelte';
	import DesignInspector from './design-inspector.svelte';
	import DisplayFrames from './display-frames.svelte';
	import FormFrames from './form-frames.svelte';
	import FoundationFrames from './foundation-frames.svelte';
	import MessagingFrames from './messaging-frames.svelte';
	import MessengerFrames from './messenger-frames.svelte';
	import NavigationFrames from './navigation-frames.svelte';
	import OverlayFrames from './overlay-frames.svelte';
	import { designText } from './text';
	import { DesignWorkspace, provideWorkspace, zoomLevels, type ThemeView } from './workspace.svelte';

	type DesignPage = { id: string; name: string; description: string; parts: Component<{ pageID: string }>[] };

	const text = createPageText(designText);
	const workspace = provideWorkspace(new DesignWorkspace());
	const languages = localeOptions.map((option) => ({ code: option.value, label: option.label }));

	const pages: DesignPage[] = $derived([
		{ id: 'foundations', name: text.foundationsPage, description: text.foundationsPageDescription, parts: [FoundationFrames] },
		{
			id: 'components',
			name: text.componentsPage,
			description: text.componentsPageDescription,
			parts: [ActionFrames, FormFrames, DisplayFrames, NavigationFrames, OverlayFrames, MessagingFrames]
		},
		{ id: 'messenger', name: text.messengerPage, description: text.messengerPageDescription, parts: [MessengerFrames] },
		{ id: 'app', name: text.appPage, description: text.appPageDescription, parts: [AppFrames] }
	]);
	const currentPage = $derived(pages.find((candidate) => candidate.id === workspace.pageID) ?? pages[0]);
	const layers = $derived(workspace.framesOf(currentPage.id));
	const layerSections = $derived([...new Set(layers.map((frame) => frame.sectionName))]);
	const zoomIndex = $derived(zoomLevels.findIndex((level) => level === workspace.zoom));

	const themeViews: { value: ThemeView; label: string; icon: typeof SunIcon }[] = $derived([
		{ value: 'light', label: text.themeLight, icon: SunIcon },
		{ value: 'dark', label: text.themeDark, icon: MoonIcon },
		{ value: 'both', label: text.themeBoth, icon: ColumnsIcon }
	]);

	function openPage(pageID: string): void {
		workspace.pageID = pageID;
		workspace.selectedID = '';
		document.getElementById('design-canvas')?.scrollTo({ top: 0, left: 0 });
	}

	function chooseThemeView(value: string): void {
		const view = themeViews.find((candidate) => candidate.value === value);
		if (view) workspace.themeView = view.value;
	}

	function chooseLanguage(code: string): void {
		const option = localeOptions.find((candidate) => candidate.value === code);
		if (option) setLocale(option.value);
	}

	async function revealFrame(id: string): Promise<void> {
		workspace.select(id);
		await tick();
		document.getElementById(id)?.scrollIntoView({ block: 'center', inline: 'center', behavior: 'smooth' });
	}

	function zoomBy(step: number): void {
		const next = zoomLevels[zoomIndex + step];
		if (next) workspace.zoom = next;
	}

	onMount(() => {
		workspace.pageID = pages[0].id;
		const linkedFrame = page.url.hash.slice(1);
		if (linkedFrame) void revealFrame(linkedFrame);
	});

	$effect(() => {
		const hash = workspace.selectedID ? `#${workspace.selectedID}` : '';
		if (page.url.hash !== hash) replaceState(`${page.url.pathname}${hash}`, page.state);
	});
</script>

<svelte:head>
	<title>{text.title} · internkim</title>
</svelte:head>

<div class="bg-background text-foreground flex h-svh flex-col overflow-hidden">
	<header class="flex h-12 shrink-0 items-center gap-2 border-b px-3">
		<span class="bg-brand text-brand-foreground grid size-6 shrink-0 place-items-center rounded-md"><FrameIcon class="size-3.5" /></span>
		<h1 class="truncate text-sm font-semibold">{text.title}</h1>
		<span class="text-muted-foreground hidden text-xs tabular-nums md:inline">{workspace.frames.length} {text.framesUnit}</span>
		<div class="ml-auto flex items-center gap-1.5">
			<ToggleGroup.Root type="single" size="sm" variant="outline" value={workspace.themeView} onValueChange={chooseThemeView} aria-label={text.themeViewLabel}>
				{#each themeViews as view (view.value)}
					<ToggleGroup.Item value={view.value} aria-label={view.label} title={view.label}><view.icon /></ToggleGroup.Item>
				{/each}
			</ToggleGroup.Root>
			<div class="hidden items-center sm:flex">
				<Button variant="ghost" size="icon-sm" aria-label={text.zoomOut} disabled={zoomIndex <= 0} onclick={() => zoomBy(-1)}><MinusIcon /></Button>
				<span class="w-10 text-center text-xs tabular-nums">{workspace.zoom}%</span>
				<Button variant="ghost" size="icon-sm" aria-label={text.zoomIn} disabled={zoomIndex >= zoomLevels.length - 1} onclick={() => zoomBy(1)}><PlusIcon /></Button>
			</div>
			<LightSwitch variant="ghost" size="sm" />
			<LanguageSwitcher {languages} value={currentLocale.value} variant="ghost" onChange={chooseLanguage} />
		</div>
	</header>

	<div class="flex shrink-0 border-b px-3 py-2 lg:hidden">
		<Select.Root type="single" value={currentPage.id} onValueChange={openPage}>
			<Select.Trigger class="w-full" aria-label={text.pagesLabel}>{currentPage.name}</Select.Trigger>
			<Select.Content>
				{#each pages as designPage (designPage.id)}
					<Select.Item value={designPage.id}>{designPage.name}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>

	<div class="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_auto] lg:grid-cols-[15rem_minmax(0,1fr)] xl:grid-cols-[15rem_minmax(0,1fr)_18rem] xl:grid-rows-1">
		<aside class="bg-sidebar text-sidebar-foreground row-span-2 hidden min-h-0 flex-col border-r lg:flex xl:row-span-1">
			<nav class="grid gap-0.5 border-b p-2" aria-label={text.pagesLabel}>
				<h2 class="text-muted-foreground px-2 pt-1 pb-1.5 text-[11px] font-medium">{text.pagesLabel}</h2>
				{#each pages as designPage (designPage.id)}
					<button
						type="button"
						class={cn(
							'hover:bg-sidebar-accent flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm',
							designPage.id === currentPage.id && 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
						)}
						aria-current={designPage.id === currentPage.id ? 'page' : undefined}
						onclick={() => openPage(designPage.id)}
					>
						<FileIcon class="text-muted-foreground size-3.5" />
						<span class="flex-1 truncate">{designPage.name}</span>
						<span class="text-muted-foreground text-[11px] tabular-nums">{workspace.framesOf(designPage.id).length}</span>
					</button>
				{/each}
			</nav>
			<nav class="min-h-0 flex-1 overflow-y-auto p-2" aria-label={text.layersLabel}>
				<h2 class="text-muted-foreground px-2 pt-1 pb-1.5 text-[11px] font-medium">{text.layersLabel}</h2>
				{#each layerSections as sectionName (sectionName)}
					<div class="grid gap-px pb-2">
						<span class="text-muted-foreground truncate px-2 py-1 text-[11px]">{sectionName}</span>
						{#each layers.filter((frame) => frame.sectionName === sectionName) as frame (frame.id)}
							<button
								type="button"
								class={cn(
									'hover:bg-sidebar-accent flex items-center gap-1.5 rounded-sm py-1 pr-2 pl-4 text-left text-xs',
									frame.id === workspace.selectedID && 'bg-brand/10 text-brand hover:bg-brand/15'
								)}
								onclick={() => revealFrame(frame.id)}
							>
								<FrameIcon class="size-3 shrink-0 opacity-60" />
								<span class="truncate">{frame.name}</span>
							</button>
						{/each}
					</div>
				{/each}
			</nav>
		</aside>

		<main
			id="design-canvas"
			class="bg-muted min-h-0 overflow-auto bg-[radial-gradient(color-mix(in_oklab,var(--foreground)_14%,transparent)_1px,transparent_1px)] bg-size-[18px_18px]"
			aria-label={currentPage.name}
		>
			<div class="grid gap-10 p-4 sm:p-8" style:zoom={workspace.zoom / 100} style:width={`${10000 / workspace.zoom}%`}>
				{#each pages as designPage (designPage.id)}
					<div class="grid gap-10" hidden={designPage.id !== currentPage.id}>
						{#each designPage.parts as Part, index (index)}
							<Part pageID={designPage.id} />
						{/each}
					</div>
				{/each}
			</div>
		</main>

		<aside
			class={cn(
				'bg-background min-h-0 overflow-y-auto border-t xl:block xl:border-t-0 xl:border-l',
				workspace.selectedFrame ? 'max-h-[40svh] xl:max-h-none' : 'hidden'
			)}
			aria-label={text.inspectorLabel}
		>
			<DesignInspector pageName={currentPage.name} pageDescription={currentPage.description} />
		</aside>
	</div>
</div>

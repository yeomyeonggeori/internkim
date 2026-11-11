<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import type { Snippet } from 'svelte';
	import { settingsPath, type SettingsSectionID } from './settings-sections';
	import { companySettingsText } from './text';

	let { section, children }: { section: SettingsSectionID; children: Snippet } = $props();

	const text = createPageText(companySettingsText);
</script>

<main class="h-full min-h-0 w-full flex-1 overflow-y-auto bg-background text-foreground max-sm:scroll-pb-[calc(var(--app-mobile-nav-height)+var(--app-mobile-nav-bottom))]">
	<div class="mx-auto grid max-w-3xl gap-6 px-4 py-6 max-sm:pb-[calc(var(--app-mobile-nav-height)+var(--app-mobile-nav-bottom)+1.5rem)] sm:px-6">
		<header class="flex items-center gap-2">
			<Button href={settingsPath(page.url.pathname)} variant="ghost" size="icon-sm" aria-label={text.title} class="sm:hidden">
				<ChevronLeftIcon />
			</Button>
			<h1 class="text-2xl font-semibold">{text.sections[section]}</h1>
		</header>
		{@render children()}
	</div>
</main>

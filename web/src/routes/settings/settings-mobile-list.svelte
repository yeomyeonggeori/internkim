<script lang="ts">
	import { page } from '$app/state';
	import * as Item from '$lib/components/ui/item';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { settingsPath, visibleSettingsGroups, type SettingsAccess } from './settings-sections';
	import { companySettingsText } from './text';

	let { access }: { access: SettingsAccess } = $props();

	const text = createPageText(companySettingsText);
	const groups = $derived(visibleSettingsGroups(access));
</script>

<main class="h-full min-h-0 w-full flex-1 overflow-y-auto bg-background text-foreground sm:hidden max-sm:scroll-pb-[calc(var(--app-mobile-nav-height)+var(--app-mobile-nav-bottom))]">
	<nav aria-label={text.title} class="grid gap-6 px-4 py-6 max-sm:pb-[calc(var(--app-mobile-nav-height)+var(--app-mobile-nav-bottom)+1.5rem)]">
		<h1 class="text-2xl font-semibold">{text.title}</h1>
		{#each groups as { group, sections } (group)}
			<section class="grid gap-2">
				<h2 class="px-2 text-xs font-medium text-muted-foreground">{text.groups[group]}</h2>
				<ul class="divide-y overflow-hidden rounded-xl border">
					{#each sections as section (section.id)}
						{@const Icon = section.icon}
						<li>
							<Item.Root size="sm">
								{#snippet child({ props })}
									<a href={settingsPath(page.url.pathname, section.id)} {...props}>
										<Item.Media><Icon /></Item.Media>
										<Item.Content><Item.Title>{text.sections[section.id]}</Item.Title></Item.Content>
										<Item.Actions><ChevronRightIcon class="text-muted-foreground" /></Item.Actions>
									</a>
								{/snippet}
							</Item.Root>
						</li>
					{/each}
				</ul>
			</section>
		{/each}
	</nav>
</main>

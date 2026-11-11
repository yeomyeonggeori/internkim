<script lang="ts">
	import { page } from '$app/state';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { settingsPath, visibleSettingsGroups, type SettingsAccess, type SettingsSectionID } from './settings-sections';
	import { companySettingsText } from './text';

	let { access, currentSectionID }: { access: SettingsAccess; currentSectionID: SettingsSectionID | null } = $props();

	const text = createPageText(companySettingsText);
	const groups = $derived(visibleSettingsGroups(access));
</script>

<nav aria-label={text.title} class="flex w-60 shrink-0 flex-col gap-2 overflow-y-auto border-r bg-sidebar py-4 text-sidebar-foreground max-sm:hidden">
	<h2 class="px-4 text-xl font-semibold">{text.title}</h2>
	{#each groups as { group, sections } (group)}
		<Sidebar.Group>
			<Sidebar.GroupLabel>{text.groups[group]}</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<Sidebar.Menu>
					{#each sections as section (section.id)}
						{@const Icon = section.icon}
						{@const isCurrent = section.id === currentSectionID}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton isActive={isCurrent}>
								{#snippet child({ props })}
									<a href={settingsPath(page.url.pathname, section.id)} aria-current={isCurrent ? 'page' : undefined} {...props}>
										<Icon />
										<span>{text.sections[section.id]}</span>
									</a>
								{/snippet}
							</Sidebar.MenuButton>
						</Sidebar.MenuItem>
					{/each}
				</Sidebar.Menu>
			</Sidebar.GroupContent>
		</Sidebar.Group>
	{/each}
</nav>

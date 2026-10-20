<script lang="ts">
	import { appRailMode, type AppRailMode } from '$lib/components/app-rail-mode.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';

	const text = createPageText(appShellText);

	const options = $derived<{ mode: AppRailMode; label: string }[]>([
		{ mode: 'expanded', label: text.sidebarExpanded },
		{ mode: 'collapsed', label: text.sidebarCollapsed },
		{ mode: 'hover', label: text.sidebarExpandOnHover }
	]);
</script>

<Sidebar.MenuItem>
	<DropdownMenu.Root bind:open={appRailMode.isMenuOpen}>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Sidebar.MenuButton {...props} tooltipContent={text.sidebar} aria-label={text.sidebar}>
					<PanelLeftIcon />
					<span>{text.sidebar}</span>
				</Sidebar.MenuButton>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content side="right" align="end" sideOffset={4} class="min-w-44 rounded-lg">
			<DropdownMenu.Label>{text.sidebar}</DropdownMenu.Label>
			<DropdownMenu.RadioGroup value={appRailMode.mode} onValueChange={appRailMode.select}>
				{#each options as option (option.mode)}
					<DropdownMenu.RadioItem value={option.mode}>{option.label}</DropdownMenu.RadioItem>
				{/each}
			</DropdownMenu.RadioGroup>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</Sidebar.MenuItem>

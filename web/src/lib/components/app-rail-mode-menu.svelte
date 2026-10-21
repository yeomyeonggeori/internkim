<script lang="ts">
	import { appRailMode, type AppRailMode } from '$lib/components/app-rail-mode.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Kbd } from '$lib/components/ui/kbd';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { mergeProps } from 'bits-ui';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';

	let { class: className = '' }: { class?: string } = $props();

	const text = createPageText(appShellText);

	const options = $derived<{ mode: AppRailMode; label: string }[]>([
		{ mode: 'expanded', label: text.sidebarExpanded },
		{ mode: 'collapsed', label: text.sidebarCollapsed },
		{ mode: 'hover', label: text.sidebarExpandOnHover }
	]);
</script>

<DropdownMenu.Root bind:open={appRailMode.isMenuOpen}>
	<DropdownMenu.Trigger>
		{#snippet child({ props: triggerProps })}
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props: tooltipProps })}
						<Button
							{...mergeProps(triggerProps, tooltipProps)}
							variant="ghost"
							size="icon-sm"
							aria-label={text.sidebar}
							class={className}
						>
							<PanelLeftIcon />
						</Button>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Content side="right">
					{text.sidebar}
					<Kbd>,</Kbd>
				</Tooltip.Content>
			</Tooltip.Root>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content side="bottom" align="start" sideOffset={4} class="min-w-44 rounded-lg">
		<DropdownMenu.Label>{text.sidebar}</DropdownMenu.Label>
		<DropdownMenu.RadioGroup value={appRailMode.mode} onValueChange={appRailMode.select}>
			{#each options as option (option.mode)}
				<DropdownMenu.RadioItem value={option.mode}>{option.label}</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>

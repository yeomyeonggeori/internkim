<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion/index.js';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import * as Command from '$lib/components/ui/command/index.js';
	import * as Pagination from '$lib/components/ui/pagination/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { sidebarMenuButtonVariants } from '$lib/components/ui/sidebar/sidebar-menu-button.svelte';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { tabsListVariants } from '$lib/components/ui/tabs/tabs-list.svelte';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import MessageCircleIcon from '@lucide/svelte/icons/message-circle';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import SquareCheckIcon from '@lucide/svelte/icons/square-check';
	import UsersIcon from '@lucide/svelte/icons/users';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';
	import { variantAxes, variantValues } from './workspace.svelte';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);
	const navigation = $derived([
		{ label: text.sample.messenger, icon: MessageCircleIcon, badge: '3', isActive: true },
		{ label: text.sample.tasks, icon: SquareCheckIcon, badge: '', isActive: false },
		{ label: text.sample.calendar, icon: CalendarIcon, badge: '', isActive: false },
		{ label: text.sample.people, icon: UsersIcon, badge: '', isActive: false }
	]);
</script>

<DesignSection {pageID} title={text.navigationSection}>
	<DesignFrame id="tabs" name="Tabs" source="ui/tabs" variants={variantAxes(tabsListVariants.variants)} class="w-96">
		{#snippet children()}
			{#each variantValues(tabsListVariants.variants.variant) as variant (variant)}
				<Tabs.Root value="week">
					<Tabs.List {variant}>
						<Tabs.Trigger value="day">{text.sample.day}</Tabs.Trigger>
						<Tabs.Trigger value="week">{text.sample.week}</Tabs.Trigger>
						<Tabs.Trigger value="month">{text.sample.month}</Tabs.Trigger>
						<Tabs.Trigger value="year" disabled>{text.sample.year}</Tabs.Trigger>
					</Tabs.List>
				</Tabs.Root>
			{/each}
		{/snippet}
	</DesignFrame>

	<DesignFrame id="underline-tabs" name="UnderlineTabs" source="ui/underline-tabs" note={text.underlineTabsNote} class="w-96">
		{#snippet children()}
			<UnderlineTabs.Root value="members">
				<UnderlineTabs.List>
					<UnderlineTabs.Trigger value="profile">{text.sample.profile}</UnderlineTabs.Trigger>
					<UnderlineTabs.Trigger value="members">{text.sample.people}</UnderlineTabs.Trigger>
					<UnderlineTabs.Trigger value="settings">{text.sample.settings}</UnderlineTabs.Trigger>
				</UnderlineTabs.List>
			</UnderlineTabs.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="breadcrumb" name="Breadcrumb" source="ui/breadcrumb" class="w-96">
		{#snippet children()}
			<Breadcrumb.Root>
				<Breadcrumb.List>
					<Breadcrumb.Item><Breadcrumb.Link href="#breadcrumb">{text.sample.dataRoom}</Breadcrumb.Link></Breadcrumb.Item>
					<Breadcrumb.Separator />
					<Breadcrumb.Item><Breadcrumb.Ellipsis /></Breadcrumb.Item>
					<Breadcrumb.Separator />
					<Breadcrumb.Item><Breadcrumb.Page>{text.sample.fileName}</Breadcrumb.Page></Breadcrumb.Item>
				</Breadcrumb.List>
			</Breadcrumb.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="pagination" name="Pagination" source="ui/pagination" class="w-[28rem]">
		{#snippet children()}
			<Pagination.Root count={50} perPage={10} page={2}>
				{#snippet children({ pages, currentPage })}
					<Pagination.Content>
						<Pagination.Item><Pagination.Previous /></Pagination.Item>
						{#each pages as page (page.key)}
							{#if page.type === 'ellipsis'}
								<Pagination.Item><Pagination.Ellipsis /></Pagination.Item>
							{:else}
								<Pagination.Item><Pagination.Link {page} isActive={currentPage === page.value}>{page.value}</Pagination.Link></Pagination.Item>
							{/if}
						{/each}
						<Pagination.Item><Pagination.Next /></Pagination.Item>
					</Pagination.Content>
				{/snippet}
			</Pagination.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="accordion" name="Accordion" source="ui/accordion" class="w-96">
		{#snippet children()}
			<Accordion.Root type="single" value="first">
				<Accordion.Item value="first">
					<Accordion.Trigger>{text.sample.faqOne}</Accordion.Trigger>
					<Accordion.Content>{text.sample.faqOneAnswer}</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="second">
					<Accordion.Trigger>{text.sample.faqTwo}</Accordion.Trigger>
					<Accordion.Content>{text.sample.faqTwoAnswer}</Accordion.Content>
				</Accordion.Item>
			</Accordion.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="collapsible" name="Collapsible" source="ui/collapsible">
		{#snippet children()}
			<Collapsible.Root open class="grid gap-2">
				<div class="flex items-center justify-between">
					<span class="text-sm font-medium">{text.sample.attachments}</span>
					<Collapsible.Trigger>
						{#snippet child({ props })}
							<Button {...props} variant="ghost" size="icon-sm" aria-label={text.sample.toggle}><ChevronsUpDownIcon /></Button>
						{/snippet}
					</Collapsible.Trigger>
				</div>
				<div class="rounded-md border px-3 py-2 text-sm">{text.sample.fileName}</div>
				<Collapsible.Content class="grid gap-2">
					<div class="rounded-md border px-3 py-2 text-sm">{text.sample.sheetName}</div>
				</Collapsible.Content>
			</Collapsible.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="sidebar" name="Sidebar" source="ui/sidebar" variants={variantAxes(sidebarMenuButtonVariants.variants)} class="h-96 w-72 p-0">
		{#snippet children()}
			<Sidebar.Provider class="min-h-0 flex-1">
				<Sidebar.Root collapsible="none" class="w-full">
					<Sidebar.Header>
						<Sidebar.Input placeholder={text.sample.search} />
					</Sidebar.Header>
					<Sidebar.Content>
						<Sidebar.Group>
							<Sidebar.GroupLabel>{text.sample.workspace}</Sidebar.GroupLabel>
							<Sidebar.GroupContent>
								<Sidebar.Menu>
									{#each navigation as item (item.label)}
										<Sidebar.MenuItem>
											<Sidebar.MenuButton isActive={item.isActive}><item.icon />{item.label}</Sidebar.MenuButton>
											{#if item.badge}<Sidebar.MenuBadge>{item.badge}</Sidebar.MenuBadge>{/if}
										</Sidebar.MenuItem>
									{/each}
									<Sidebar.MenuItem><Sidebar.MenuSkeleton showIcon /></Sidebar.MenuItem>
								</Sidebar.Menu>
							</Sidebar.GroupContent>
						</Sidebar.Group>
						<Sidebar.Separator />
						<Sidebar.Group>
							<Sidebar.GroupContent>
								<Sidebar.Menu>
									<Sidebar.MenuItem>
										<Sidebar.MenuButton variant="outline"><InboxIcon />{text.sample.inbox}</Sidebar.MenuButton>
										<Sidebar.MenuSub>
											<Sidebar.MenuSubItem><Sidebar.MenuSubButton isActive>{text.sample.unread}</Sidebar.MenuSubButton></Sidebar.MenuSubItem>
											<Sidebar.MenuSubItem><Sidebar.MenuSubButton>{text.sample.archived}</Sidebar.MenuSubButton></Sidebar.MenuSubItem>
										</Sidebar.MenuSub>
									</Sidebar.MenuItem>
								</Sidebar.Menu>
							</Sidebar.GroupContent>
						</Sidebar.Group>
					</Sidebar.Content>
					<Sidebar.Footer>
						<Sidebar.Menu>
							<Sidebar.MenuItem><Sidebar.MenuButton size="sm"><SettingsIcon />{text.sample.settings}</Sidebar.MenuButton></Sidebar.MenuItem>
						</Sidebar.Menu>
					</Sidebar.Footer>
				</Sidebar.Root>
			</Sidebar.Provider>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="command" name="Command" source="ui/command" class="w-96">
		{#snippet children()}
			<Command.Root class="rounded-lg border" disableInitialScroll>
				<Command.Input placeholder={text.sample.commandPlaceholder} />
				<Command.List>
					<Command.Empty>{text.sample.noResults}</Command.Empty>
					<Command.Group heading={text.sample.workspace}>
						<Command.Item><MessageCircleIcon />{text.sample.messenger}<Command.Shortcut>⌘1</Command.Shortcut></Command.Item>
						<Command.Item><SquareCheckIcon />{text.sample.tasks}<Command.Shortcut>⌘2</Command.Shortcut></Command.Item>
						<Command.LinkItem href="#command"><CalendarIcon />{text.sample.calendar}</Command.LinkItem>
					</Command.Group>
					<Command.Separator />
					<Command.Group heading={text.sample.settings}>
						<Command.Item disabled><SettingsIcon />{text.sample.settings}</Command.Item>
					</Command.Group>
				</Command.List>
			</Command.Root>
		{/snippet}
	</DesignFrame>
</DesignSection>

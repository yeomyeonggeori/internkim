<script lang="ts">
	import type { DialogContentProps } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Drawer from '$lib/components/ui/drawer/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as HoverCard from '$lib/components/ui/hover-card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);

	type StaysOpen = Pick<
		DialogContentProps,
		'trapFocus' | 'preventScroll' | 'interactOutsideBehavior' | 'escapeKeydownBehavior' | 'onOpenAutoFocus'
	> & { portalProps: { disabled: boolean } };

	const staysOpen: StaysOpen = {
		portalProps: { disabled: true },
		trapFocus: false,
		preventScroll: false,
		interactOutsideBehavior: 'ignore',
		escapeKeydownBehavior: 'ignore',
		onOpenAutoFocus: (event) => event.preventDefault()
	};

	function askToDelete(): void {
		confirmDelete({
			title: text.sample.deleteTitle,
			description: text.sample.deleteDescription,
			onConfirm: async () => {}
		});
	}
</script>

{#snippet menuItems(Group: typeof DropdownMenu | typeof ContextMenu)}
	<Group.Label>{text.sample.message}</Group.Label>
	<Group.Item><ReplyIcon />{text.sample.reply}<Group.Shortcut>R</Group.Shortcut></Group.Item>
	<Group.Item><CopyIcon />{text.sample.copy}<Group.Shortcut>⌘C</Group.Shortcut></Group.Item>
	<Group.Item><PencilIcon />{text.sample.edit}</Group.Item>
	<Group.CheckboxItem checked>{text.sample.notify}</Group.CheckboxItem>
	<Group.Separator />
	<Group.Item variant="destructive"><TrashIcon />{text.sample.remove}</Group.Item>
{/snippet}

<DesignSection {pageID} title={text.overlaySection} description={text.overlaySectionDescription}>
	<DesignFrame id="dialog" name="Dialog" source="ui/dialog" isContained class="h-80 w-[30rem]">
		{#snippet children()}
			<Dialog.Root open>
				<Dialog.Content {...staysOpen} showCloseButton>
					<Dialog.Header>
						<Dialog.Title>{text.sample.dialogTitle}</Dialog.Title>
						<Dialog.Description>{text.sample.dialogDescription}</Dialog.Description>
					</Dialog.Header>
					<Input value="이샘플" />
					<Dialog.Footer>
						<Button variant="outline">{text.sample.cancel}</Button>
						<Button>{text.sample.save}</Button>
					</Dialog.Footer>
				</Dialog.Content>
			</Dialog.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="alert-dialog" name="AlertDialog" source="ui/alert-dialog" isContained class="h-72 w-[30rem]">
		{#snippet children()}
			<AlertDialog.Root open>
				<AlertDialog.Content {...staysOpen}>
					<AlertDialog.Header>
						<AlertDialog.Title>{text.sample.deleteTitle}</AlertDialog.Title>
						<AlertDialog.Description>{text.sample.deleteDescription}</AlertDialog.Description>
					</AlertDialog.Header>
					<AlertDialog.Footer>
						<AlertDialog.Cancel>{text.sample.cancel}</AlertDialog.Cancel>
						<AlertDialog.Action variant="destructive">{text.sample.remove}</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="confirm-delete-dialog" name="ConfirmDeleteDialog" source="ui/confirm-delete-dialog" note={text.liveOverlayNote}>
		{#snippet children()}
			<Button variant="destructive" onclick={askToDelete}><TrashIcon data-icon="inline-start" />{text.sample.remove}</Button>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="sheet" name="Sheet" source="ui/sheet" isContained class="h-80 w-[30rem]">
		{#snippet children()}
			<Sheet.Root open>
				<Sheet.Content {...staysOpen} side="right" class="w-64">
					<Sheet.Header>
						<Sheet.Title>{text.sample.threadTitle}</Sheet.Title>
						<Sheet.Description>{text.sample.threadDescription}</Sheet.Description>
					</Sheet.Header>
				</Sheet.Content>
			</Sheet.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="drawer" name="Drawer" source="ui/drawer" isContained class="h-80 w-[24rem]">
		{#snippet children()}
			<Drawer.Root open modal={false}>
				<Drawer.Content portalProps={{ disabled: true }}>
					<Drawer.Header>
						<Drawer.Title>{text.sample.drawerTitle}</Drawer.Title>
						<Drawer.Description>{text.sample.drawerDescription}</Drawer.Description>
					</Drawer.Header>
					<Drawer.Footer><Button>{text.sample.save}</Button></Drawer.Footer>
				</Drawer.Content>
			</Drawer.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="popover" name="Popover" source="ui/popover" isContained class="h-64 w-80">
		{#snippet children()}
			<Popover.Root open>
				<Popover.Trigger>
					{#snippet child({ props })}<Button {...props} variant="outline" class="w-fit">{text.sample.open}</Button>{/snippet}
				</Popover.Trigger>
				<Popover.Content {...staysOpen} align="start" avoidCollisions={false}>
					<Popover.Header>
						<Popover.Title>{text.sample.popoverTitle}</Popover.Title>
						<Popover.Description>{text.sample.popoverDescription}</Popover.Description>
					</Popover.Header>
				</Popover.Content>
			</Popover.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="hover-card" name="HoverCard" source="ui/hover-card" isContained class="h-56 w-80">
		{#snippet children()}
			<HoverCard.Root open>
				<HoverCard.Trigger class="w-fit text-sm font-medium underline underline-offset-4">@이샘플</HoverCard.Trigger>
				<HoverCard.Content portalProps={{ disabled: true }} align="start" side="bottom" avoidCollisions={false}>
					<div class="flex gap-3">
						<PersonAvatar name="이샘플" seed="design-hover" class="size-10" />
						<div class="grid gap-0.5 text-sm">
							<span class="font-medium">이샘플</span>
							<span class="text-muted-foreground text-xs">sample@example.com</span>
						</div>
					</div>
				</HoverCard.Content>
			</HoverCard.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="tooltip" name="Tooltip" source="ui/tooltip" note={text.liveOverlayNote} class="h-32 w-64">
		{#snippet children()}
			<Tooltip.Provider>
				<Tooltip.Root>
					<Tooltip.Trigger class="mt-auto w-fit">
						{#snippet child({ props })}<Button {...props} variant="outline" size="sm">{text.sample.hoverMe}</Button>{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content portalProps={{ disabled: true }} side="top" avoidCollisions={false}>{text.sample.tooltip}</Tooltip.Content>
				</Tooltip.Root>
			</Tooltip.Provider>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="dropdown-menu" name="DropdownMenu" source="ui/dropdown-menu" isContained class="h-80 w-72">
		{#snippet children()}
			<DropdownMenu.Root open>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}<Button {...props} variant="outline" class="w-fit">{text.sample.more}</Button>{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content {...staysOpen} align="start" avoidCollisions={false} class="w-52">
					{@render menuItems(DropdownMenu)}
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="context-menu" name="ContextMenu" source="ui/context-menu" note={text.liveOverlayNote}>
		{#snippet children()}
			<ContextMenu.Root>
				<ContextMenu.Trigger class="text-muted-foreground flex h-28 items-center justify-center rounded-lg border border-dashed text-sm">
					{text.sample.rightClick}
				</ContextMenu.Trigger>
				<ContextMenu.Content class="w-52">
					{@render menuItems(ContextMenu)}
				</ContextMenu.Content>
			</ContextMenu.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="sonner" name="Sonner" source="ui/sonner" note={text.liveOverlayNote}>
		{#snippet children()}
			<div class="flex flex-wrap gap-2">
				<Button variant="outline" size="sm" onclick={() => toast(text.sample.toastDefault)}>default</Button>
				<Button variant="outline" size="sm" onclick={() => toast.success(text.sample.toastSuccess)}>success</Button>
				<Button variant="outline" size="sm" onclick={() => toast.error(text.sample.toastError)}>error</Button>
				<Button variant="outline" size="sm" onclick={() => toast.info(text.sample.toastInfo)}>info</Button>
				<Button variant="outline" size="sm" onclick={() => toast.warning(text.sample.toastWarning)}>warning</Button>
				<Button variant="outline" size="sm" onclick={() => toast.loading(text.sample.saving)}>loading</Button>
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

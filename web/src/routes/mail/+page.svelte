<script lang="ts">
	import AppFloatingActionButton from '$lib/components/app-floating-action-button.svelte';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { page as navigationPage } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { onMount } from 'svelte';
	import MailComposeSheet from './mail-compose-sheet.svelte';
	import MailMessageDetail from './mail-message-detail.svelte';
	import MailMessageList from './mail-message-list.svelte';
	import { createMailPageController } from './mail-page-controller.svelte';
	import MailSettingsSheet from './mail-settings-sheet.svelte';
	import MailSidebar from './mail-sidebar.svelte';
	import { mailText } from './text';

	const text = createPageText(mailText);
	const page = createMailPageController(text);
	const requestedMessage = $derived(requestedMailboxMessage(navigationPage.url.searchParams));
	let appliedMessageKey = '';

	function requestedMailboxMessage(searchParams: URLSearchParams) {
		const mailbox = searchParams.get('mailbox') ?? '';
		const uid = Number(searchParams.get('uid'));
		if (!mailbox || !Number.isFinite(uid) || uid <= 0) return null;
		return { mailbox, uid };
	}

	const fitsWideSidebar = new MediaQuery('min-width: 1280px');
	const fitsTwoPanes = new MediaQuery('min-width: 1024px');
	const fitsSidebarRail = new MediaQuery('min-width: 640px');
	const isDetailInline = $derived(fitsTwoPanes.current);
	const isStackedDetail = $derived(!isDetailInline && page.selectedMessage !== null);
	const paneColumns = $derived(isDetailInline ? 'grid-cols-[minmax(300px,380px)_minmax(0,1fr)]' : 'grid-cols-1');

	let isMailboxSidebarOpen = $state(true);

	$effect(() => {
		isMailboxSidebarOpen = fitsWideSidebar.current;
	});

	$effect(() => {
		page.canSelectFirstMessage = isDetailInline;
	});

	$effect(() => {
		breadcrumbMeta.value = page.selectedMailboxLabel();
		return () => {
			breadcrumbMeta.value = '';
		};
	});

	$effect(() => {
		if (!page.hasLoadedAccount || !requestedMessage) return;
		const messageKey = `${requestedMessage.mailbox}:${requestedMessage.uid}`;
		if (messageKey === appliedMessageKey) return;
		appliedMessageKey = messageKey;
		void page.openMailboxMessage(requestedMessage.mailbox, requestedMessage.uid);
	});

	onMount(() => {
		void page.loadMail();
		return pageActions.setRefresh(page.loadMail);
	});
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<Sidebar.Provider
	bind:open={isMailboxSidebarOpen}
	class="relative h-[calc(100svh-48px)] min-h-0 w-full transform-gpu overflow-hidden bg-background text-foreground"
>
	{@render mailboxSidebar()}

	<main class="grid min-w-0 flex-1 overflow-hidden {paneColumns}">
		{#if !isStackedDetail}
			<MailMessageList
				account={page.account}
				selectedMailboxCountText={page.selectedMailboxCountText()}
				isLoading={page.isLoading}
				isSyncing={page.isSyncing}
				hasLoadedAccount={page.hasLoadedAccount}
				isLoadingMessages={page.isLoadingMessages}
				isUnreadOnly={page.isUnreadOnly}
				canLoadMoreMessages={page.canLoadMoreMessages()}
				errorMessage={page.errorMessage}
				messages={page.visibleMessages()}
				selectedMessage={page.selectedMessage}
				hasMailboxTrigger={!fitsSidebarRail.current}
				{text}
				openSettings={page.openSettings}
				loadMoreMessages={page.loadMoreMessages}
				setUnreadOnly={page.setUnreadOnly}
				selectMessage={page.selectMessage}
			/>
		{/if}

		{#if isDetailInline}
			{@render messageDetail()}
		{:else if isStackedDetail}
			{@render messageDetail(page.clearSelectedMessage)}
		{/if}
	</main>

	<AppFloatingActionButton
		label={text.compose}
		disabled={!page.hasLoadedAccount || !page.account.isConfigured}
		onclick={page.openCompose}
	>
		<PencilIcon />
	</AppFloatingActionButton>
</Sidebar.Provider>

{#snippet mailboxSidebar()}
	<MailSidebar
		account={page.account}
		mailboxes={page.pageMailboxes()}
		selectedMailbox={page.selectedMailbox}
		hasLoadedAccount={page.hasLoadedAccount}
		isLoadingMailboxes={page.isLoadingMailboxes}
		{text}
		openSettings={page.openSettings}
		selectMailbox={(mailboxName) => {
			if (!isDetailInline) page.clearSelectedMessage();
			page.selectMailbox(mailboxName);
		}}
	/>
{/snippet}

{#snippet messageDetail(goBack?: () => void)}
	<MailMessageDetail
		account={page.account}
		selectedMessage={page.selectedMessage}
		hasLoadedAccount={page.hasLoadedAccount}
		isLoadingMessage={page.isLoadingMessage}
		messageBody={page.selectedMessageBody()}
		messageBodyHTML={page.selectedMessageBodyHTML()}
		{text}
		moveSelectedMessage={page.moveSelectedMessage}
		openReply={page.openReply}
		openForward={page.openForward}
		openCompose={page.openCompose}
		openSettings={page.openSettings}
		{goBack}
	/>
{/snippet}

<MailSettingsSheet
	bind:open={page.isSettingsOpen}
	bind:accountDraft={page.accountDraft}
	account={page.account}
	settingsMessage={page.settingsMessage}
	isSavingAccount={page.isSavingAccount}
	isTestingAccount={page.isTestingAccount}
	{text}
	saveAccount={page.saveAccount}
	testAccount={page.testAccount}
/>

<MailComposeSheet
	bind:open={page.isComposeOpen}
	bind:composeDraft={page.composeDraft}
	composeMessage={page.composeMessage}
	isSending={page.isSending}
	fromAddress={page.account.fromAddress || page.account.email}
	focusField={page.composeFocusField}
	{text}
	sendMessage={page.sendMessage}
/>

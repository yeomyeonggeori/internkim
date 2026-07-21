<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
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

	onMount(page.loadMail);
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<main class="grid h-[calc(100svh-48px)] min-h-0 w-full flex-1 grid-cols-[240px_minmax(320px,380px)_minmax(0,1fr)] overflow-hidden bg-background text-foreground max-lg:grid-cols-[260px_minmax(0,1fr)] max-md:grid-cols-1">
	<MailSidebar
		account={page.account}
		mailboxes={page.pageMailboxes()}
		selectedMailbox={page.selectedMailbox}
		hasLoadedAccount={page.hasLoadedAccount}
		isLoadingMailboxes={page.isLoadingMailboxes}
		{text}
		openSettings={page.openSettings}
		openCompose={page.openCompose}
		selectMailbox={page.selectMailbox}
	/>

	<MailMessageList
		account={page.account}
		selectedMailbox={page.selectedMailbox}
		selectedMailboxCountText={page.selectedMailboxCountText()}
		isLoading={page.isLoading}
		isSyncing={page.isSyncing}
		hasLoadedAccount={page.hasLoadedAccount}
		isLoadingMessages={page.isLoadingMessages}
		bind:searchText={page.searchText}
		isUnreadOnly={page.isUnreadOnly}
		messagePageIndex={page.messagePageIndex}
		canPreviousMessagePage={page.canPreviousMessagePage()}
		canNextMessagePage={page.canNextMessagePage()}
		errorMessage={page.errorMessage}
		messages={page.visibleMessages()}
		selectedMessage={page.selectedMessage}
		{text}
		openSettings={page.openSettings}
		loadMail={page.loadMail}
		loadMessages={page.loadMessages}
		loadPreviousMessages={page.loadPreviousMessages}
		loadNextMessages={page.loadNextMessages}
		setUnreadOnly={page.setUnreadOnly}
		selectMessage={page.selectMessage}
	/>

	<MailMessageDetail
		account={page.account}
		selectedMailbox={page.selectedMailbox}
		selectedMessage={page.selectedMessage}
		hasLoadedAccount={page.hasLoadedAccount}
		isLoadingMessage={page.isLoadingMessage}
		messageBody={page.selectedMessageBody()}
		messageBodyHTML={page.selectedMessageBodyHTML()}
		{text}
		moveSelectedMessage={page.moveSelectedMessage}
		toggleSelectedMessageRead={page.toggleSelectedMessageRead}
		openReply={page.openReply}
		openCompose={page.openCompose}
		openSettings={page.openSettings}
	/>
</main>

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
	{text}
	sendMessage={page.sendMessage}
/>

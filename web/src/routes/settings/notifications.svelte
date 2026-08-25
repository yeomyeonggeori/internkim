<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Switch } from '$lib/components/ui/switch';
	import { Label } from '$lib/components/ui/label';
	import {
		notificationCategories,
		readNotificationSettings,
		type NotificationCategory,
		type NotificationSettings
	} from '$lib/notifications/categories';
	import { chooseNotificationSettings, myNotificationSettings } from '$lib/notifications/settings';
	import {
		reachability,
		startBeingReached,
		stopBeingReached,
		type Reachability
	} from '$lib/notifications/subscribe';
	import { sendTestNotification } from '$lib/notifications/self-test';
	import BellIcon from '@lucide/svelte/icons/bell';
	import SendIcon from '@lucide/svelte/icons/send';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let reach = $state<Reachability>('off');
	let settings = $state<NotificationSettings>(readNotificationSettings({}));
	let isLoading = $state(true);
	let isSwitching = $state(false);
	let isTesting = $state(false);

	const categoryLabels: Record<NotificationCategory, string> = $derived({
		message: text.notifyMessage,
		task: text.notifyTask,
		approval: text.notifyApproval,
		attendance: text.notifyAttendance
	});

	async function load() {
		try {
			reach = await reachability();
			settings = await myNotificationSettings();
		} catch {
			toast.error(text.notifyLoadFailed);
		} finally {
			isLoading = false;
		}
	}

	onMount(load);

	async function switchReach() {
		isSwitching = true;
		try {
			reach = reach === 'on' ? await stopBeingReached() : await startBeingReached();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.notifyFailed);
		} finally {
			isSwitching = false;
		}
	}

	async function sendTest() {
		isTesting = true;
		try {
			const { reached } = await sendTestNotification(text.notifyTestTitle, text.notifyTestBody);
			if (reached === 0) toast.error(text.notifyTestNoDevice);
			else toast.success(text.notifyTestSent);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.notifyTestFailed);
		} finally {
			isTesting = false;
		}
	}

	async function choose(category: NotificationCategory, wanted: boolean) {
		const previous = settings;
		settings = { ...settings, [category]: wanted };
		try {
			await chooseNotificationSettings(settings);
		} catch {
			settings = previous;
			toast.error(text.notifyFailed);
		}
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.notifications}</Card.Title>
		<Card.Description>{text.notificationsDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if reach === 'unsupported'}
			<p class="text-sm text-muted-foreground">{text.notifyUnsupported}</p>
		{:else if reach === 'unconfigured'}
			<p class="text-sm text-muted-foreground">{text.notifyUnconfigured}</p>
		{:else if !isLoading}
			<Button class="w-full gap-2" onclick={switchReach} disabled={isSwitching}>
				<BellIcon class="size-4" />
				{reach === 'on' ? text.notifyStop : text.notifyStart}
			</Button>
			{#if reach === 'on'}
				<Button variant="outline" class="w-full gap-2" onclick={sendTest} disabled={isTesting}>
					<SendIcon class="size-4" />
					{text.notifyTest}
				</Button>
			{/if}
			{#if reach === 'blocked'}
				<p class="text-sm text-muted-foreground">{text.notifyBlocked}</p>
			{/if}
			<div class="grid gap-3" class:opacity-50={reach !== 'on'}>
				{#each notificationCategories as category (category)}
					<div class="flex items-center justify-between gap-4">
						<Label for="{fieldID}-{category}" class="text-sm font-normal">{categoryLabels[category]}</Label>
						<Switch
							id="{fieldID}-{category}"
							checked={settings[category]}
							disabled={reach !== 'on'}
							onCheckedChange={(wanted) => choose(category, wanted)}
						/>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>

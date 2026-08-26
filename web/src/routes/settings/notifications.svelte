<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Switch } from '$lib/components/ui/switch';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		readNotificationSettings,
		readTimeOfDay,
		type NotificationCategory,
		type NotificationSettings
	} from '$lib/notifications/categories';
	import { categoriesChoosableBy } from '$lib/notifications/choosable-categories';
	import { chooseNotificationSettings, myNotificationSettings } from '$lib/notifications/settings';
	import { supabaseMemberRole } from '$lib/supabase-session';
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
	let isAdmin = $state(false);
	let settings = $state<NotificationSettings>(readNotificationSettings({}));
	let isLoading = $state(true);
	let isSwitching = $state(false);
	let isTesting = $state(false);

	const categoryLabels: Record<NotificationCategory, string> = $derived({
		message: text.notifyMessage,
		task: text.notifyTask,
		approval: text.notifyApproval,
		attendance: text.notifyAttendance,
		leave: text.notifyLeave,
		calendar: text.notifyCalendar,
		mail: text.notifyMail
	});

	async function load() {
		try {
			reach = await reachability();
			settings = await myNotificationSettings();
			isAdmin = (await supabaseMemberRole()) === 'admin';
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
		await keep({ ...settings, categories: { ...settings.categories, [category]: wanted } });
	}

	async function chooseCalendarAt(said: string) {
		const at = readTimeOfDay(said);
		if (!at || at === settings.calendarAt) return;
		await keep({ ...settings, calendarAt: at });
	}

	async function keep(wanted: NotificationSettings) {
		const previous = settings;
		settings = wanted;
		try {
			await chooseNotificationSettings(wanted);
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
				{#each categoriesChoosableBy(isAdmin) as category (category)}
					<div class="flex items-center justify-between gap-4">
						<Label for="{fieldID}-{category}" class="text-sm font-normal">{categoryLabels[category]}</Label>
						<div class="flex items-center gap-2">
							{#if category === 'calendar'}
								<Input
									type="time"
									class="h-8 w-28"
									value={settings.calendarAt}
									disabled={reach !== 'on' || !settings.categories.calendar}
									aria-label={text.notifyCalendarAt}
									onchange={(event) => chooseCalendarAt(event.currentTarget.value)}
								/>
							{/if}
							<Switch
								id="{fieldID}-{category}"
								checked={settings.categories[category]}
								disabled={reach !== 'on'}
								onCheckedChange={(wanted) => choose(category, wanted)}
							/>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>

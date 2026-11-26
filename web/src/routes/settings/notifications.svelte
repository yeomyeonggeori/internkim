<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import { Label } from '$lib/components/ui/label';
	import {
		attendanceReaches,
		isAttendanceReach,
		isTaskReach,
		taskReaches,
		type NotificationCategory,
		type TaskReach
	} from '$lib/notifications/categories';
	import {
		chooseAttendanceReach,
		chooseNotificationCategory,
		chooseTaskReach,
		type NotificationSettings
	} from '$lib/notifications/settings';
	import {
		startBeingReached,
		stopBeingReached,
		type Reachability
	} from '$lib/notifications/subscribe';
	import { hasNativePush } from '$lib/notifications/native-device';
	import { sendTestNotification } from '$lib/notifications/self-test';
	import BellIcon from '@lucide/svelte/icons/bell';
	import SendIcon from '@lucide/svelte/icons/send';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { notificationReachabilityRead, notificationSettingsRead } from './settings-reads';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let reach = $state<Reachability>(notificationReachabilityRead.value ?? 'off');
	let settings = $state<NotificationSettings>(
		notificationSettingsRead.value ?? { categories: [], attendanceReach: 'everyone', taskReach: 'team', mutedConversationIDs: [] }
	);
	let isLoadingReachability = $state(notificationReachabilityRead.value === undefined);
	let isLoadingSettings = $state(notificationSettingsRead.value === undefined);
	let reachabilityFailed = $state(false);
	let isDisposed = false;
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

	const reachOff = 'off';

	type ReachCategory = 'attendance' | 'task';
	type ReachOption = TaskReach | typeof reachOff;

	const reachLabels: Record<ReachOption, string> = $derived({
		everyone: text.notifyReachEveryone,
		teamAndSupervisors: text.notifyReachTeamAndSupervisors,
		team: text.notifyReachTeam,
		involved: text.notifyReachInvolved,
		off: text.notifyReachOff
	});

	const reachOptions: Record<ReachCategory, readonly ReachOption[]> = {
		attendance: [...attendanceReaches, reachOff],
		task: [...taskReaches, reachOff]
	};

	function reachRank(category: NotificationCategory): number {
		if (category === 'attendance') return 0;
		return category === 'task' ? 1 : 2;
	}

	function hasReach(category: NotificationCategory): category is ReachCategory {
		return category === 'attendance' || category === 'task';
	}

	const shownCategories = $derived(
		settings.categories
			.filter((choice) => choice.isChoosable)
			.toSorted((left, right) => reachRank(left.category) - reachRank(right.category))
	);

	async function loadReachability() {
		try {
			const next = await notificationReachabilityRead.read();
			if (!isDisposed) reach = next;
		} catch {
			if (!isDisposed) {
				reachabilityFailed = true;
				toast.error(text.notifyLoadFailed);
			}
		} finally {
			if (!isDisposed) isLoadingReachability = false;
		}
	}

	async function loadSettings() {
		try {
			const next = await notificationSettingsRead.read();
			if (!isDisposed) settings = next;
		} catch {
			if (!isDisposed) toast.error(text.notifyLoadFailed);
		} finally {
			if (!isDisposed) isLoadingSettings = false;
		}
	}

	onMount(() => {
		void loadReachability();
		void loadSettings();
		return () => { isDisposed = true; };
	});

	async function switchReach() {
		isSwitching = true;
		try {
			reach = reach === 'on' ? await stopBeingReached() : await startBeingReached();
			notificationReachabilityRead.remember(reach);
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
		settings = {
			...settings,
			categories: settings.categories.map((choice) =>
				choice.category === category ? { ...choice, isOn: wanted } : choice
			)
		};
		try {
			settings = await chooseNotificationCategory(category, wanted);
			notificationSettingsRead.remember(settings);
		} catch {
			settings = previous;
			toast.error(text.notifyFailed);
		}
	}

	function reachChosen(category: ReachCategory, isOn: boolean): ReachOption {
		if (!isOn) return reachOff;
		return category === 'attendance' ? settings.attendanceReach : settings.taskReach;
	}

	function withReach(category: ReachCategory, chosen: string): NotificationSettings | undefined {
		if (category === 'attendance' && isAttendanceReach(chosen)) return { ...settings, attendanceReach: chosen };
		if (category === 'task' && isTaskReach(chosen)) return { ...settings, taskReach: chosen };
		return undefined;
	}

	function saveReach(category: ReachCategory, chosen: NotificationSettings): Promise<NotificationSettings> {
		return category === 'attendance' ? chooseAttendanceReach(chosen.attendanceReach) : chooseTaskReach(chosen.taskReach);
	}

	async function chooseReach(category: ReachCategory, chosen: string) {
		if (chosen === reachOff) return choose(category, false);
		const narrowed = withReach(category, chosen);
		if (!narrowed) return;
		const previous = settings;
		settings = {
			...narrowed,
			categories: settings.categories.map((choice) =>
				choice.category === category ? { ...choice, isOn: true } : choice
			)
		};
		try {
			settings = await saveReach(category, narrowed);
			notificationSettingsRead.remember(settings);
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
		{#if isLoadingReachability}<div role="status" aria-label={text.notifications} aria-busy="true"><Skeleton aria-hidden="true" class="h-9 w-full" /></div>{/if}
		{#if reach === 'unsupported'}
			<p class="text-sm text-muted-foreground">{text.notifyUnsupported}</p>
		{:else if reach === 'unconfigured'}
			<p class="text-sm text-muted-foreground">{text.notifyUnconfigured}</p>
		{:else if !isLoadingReachability && !reachabilityFailed}
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
				<p class="text-sm text-muted-foreground">
					{hasNativePush() ? text.notifyBlockedOnDevice : text.notifyBlocked}
				</p>
			{/if}
		{/if}
		{#if isLoadingSettings}
			<div role="status" aria-label={text.notifications} aria-busy="true" class="grid gap-3">{#each [0, 1, 2] as row (row)}<div aria-hidden="true" class="flex items-center justify-between gap-4"><Skeleton class="h-4 w-40 max-w-[70%]" /><Skeleton class="h-5 w-9 rounded-full" /></div>{/each}</div>
		{:else}
			<div class="grid gap-3" class:opacity-50={isLoadingReachability || reach !== 'on'}>
				{#each shownCategories as choice (choice.category)}
					<div class="flex items-center justify-between gap-4">
						<Label for="{fieldID}-{choice.category}" class="text-sm font-normal">
							{categoryLabels[choice.category]}
						</Label>
						<div class="flex items-center gap-2">
							{#if hasReach(choice.category)}
								{@const category = choice.category}
								<Select.Root
									type="single"
									value={reachChosen(category, choice.isOn)}
									onValueChange={(chosen) => chooseReach(category, chosen)}
									disabled={isLoadingReachability || reachabilityFailed || reach !== 'on'}
								>
									<Select.Trigger id="{fieldID}-{choice.category}" class="w-44">
										{reachLabels[reachChosen(category, choice.isOn)]}
									</Select.Trigger>
									<Select.Content><Select.Group>
										{#each reachOptions[category] as option (option)}
											<Select.Item value={option} label={reachLabels[option]} />
										{/each}
									</Select.Group></Select.Content>
								</Select.Root>
							{:else}
								<Switch
									id="{fieldID}-{choice.category}"
									checked={choice.isOn}
									disabled={isLoadingReachability || reachabilityFailed || reach !== 'on'}
									onCheckedChange={(wanted) => choose(choice.category, wanted)}
								/>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>

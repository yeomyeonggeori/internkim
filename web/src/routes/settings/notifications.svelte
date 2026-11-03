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
		type AttendanceReach,
		type NotificationCategory
	} from '$lib/notifications/categories';
	import {
		chooseAttendanceReach,
		chooseNotificationCategory,
		myNotificationSettings,
		type NotificationSettings
	} from '$lib/notifications/settings';
	import {
		reachability,
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
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let reach = $state<Reachability>('off');
	let settings = $state<NotificationSettings>({ categories: [], attendanceReach: 'everyone', mutedConversationIDs: [] });
	let isLoadingReachability = $state(true);
	let isLoadingSettings = $state(true);
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

	const attendanceOff = 'off';

	const attendanceReachLabels: Record<AttendanceReach | typeof attendanceOff, string> = $derived({
		everyone: text.notifyAttendanceEveryone,
		teamAndSupervisors: text.notifyAttendanceTeamAndSupervisors,
		team: text.notifyAttendanceTeam,
		off: text.notifyAttendanceOff
	});

	const attendanceOptions = [...attendanceReaches, attendanceOff] as const;

	async function loadReachability() {
		try {
			const next = await reachability();
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
			const next = await myNotificationSettings();
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
		} catch {
			settings = previous;
			toast.error(text.notifyFailed);
		}
	}

	function attendanceChosen(isOn: boolean): AttendanceReach | typeof attendanceOff {
		return isOn ? settings.attendanceReach : attendanceOff;
	}

	async function chooseAttendance(chosen: string) {
		if (chosen === attendanceOff) return choose('attendance', false);
		if (!isAttendanceReach(chosen)) return;
		const previous = settings;
		settings = {
			...settings,
			attendanceReach: chosen,
			categories: settings.categories.map((choice) =>
				choice.category === 'attendance' ? { ...choice, isOn: true } : choice
			)
		};
		try {
			settings = await chooseAttendanceReach(chosen);
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
				{#each settings.categories.filter((choice) => choice.isChoosable) as choice (choice.category)}
					<div class="flex items-center justify-between gap-4">
						<Label for="{fieldID}-{choice.category}" class="text-sm font-normal">
							{categoryLabels[choice.category]}
						</Label>
						<div class="flex items-center gap-2">
							{#if choice.category === 'attendance'}
								<Select.Root
									type="single"
									value={attendanceChosen(choice.isOn)}
									onValueChange={chooseAttendance}
									disabled={isLoadingReachability || reachabilityFailed || reach !== 'on'}
								>
									<Select.Trigger id="{fieldID}-{choice.category}" class="w-44">
										{attendanceReachLabels[attendanceChosen(choice.isOn)]}
									</Select.Trigger>
									<Select.Content><Select.Group>
										{#each attendanceOptions as option (option)}
											<Select.Item value={option} label={attendanceReachLabels[option]} />
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

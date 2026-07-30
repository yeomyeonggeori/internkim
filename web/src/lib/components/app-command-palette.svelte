<script lang="ts">
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import { attendanceClock } from '$lib/components/attendance-clock.svelte';
	import { calendarEventSearch } from '$lib/components/calendar-event-search.svelte';
	import CalendarEventListCard from '../../routes/calendar/embed/calendar-event-list-card.svelte';
	import type { CalendarSearchResult } from '../../routes/calendar/embed/calendar-search';
	import { mailMessageSearch } from '$lib/components/mail-message-search.svelte';
	import { flowTaskSearch } from '$lib/components/flow-task-search.svelte';
	import FlowTaskBoardCard from '../../routes/flow/flow-task-board-card.svelte';
	import { flowText } from '../../routes/flow/text';
	import MailMessageRow from '../../routes/mail/mail-message-row.svelte';
	import { mailText } from '../../routes/mail/text';
	import * as Command from '$lib/components/ui/command/index.js';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../../routes/attendance/text';
	import { koreanSearchScore } from '$lib/korean-search';
	import { goto } from '$app/navigation';
	import { calendarNavigation } from '../../routes/calendar/refresh-signal.svelte';
	import { dateKeyFromDate } from '../../routes/calendar/embed/calendar-month-selection';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import MailIcon from '@lucide/svelte/icons/mail';
	import type { MailMessage } from '../../routes/mail/mail-types';
	import type { FlowTask } from '../../routes/flow/flow-types';
	import CircleIcon from '@lucide/svelte/icons/circle';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import PowerOffIcon from '@lucide/svelte/icons/power-off';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const text = createPageText(appShellText);
	const attendanceLabels = createPageText(attendanceText);
	const flowLabels = createPageText(flowText);
	const mailLabels = createPageText(mailText);
	const clockOutShortcut = '0';
	const locationShortcuts = ['1', '2', '3', '4', '5', '6', '7', '8', '9'];
	let searchValue = $state('');

	const calendarResults = $derived(calendarEventSearch.search(searchValue));
	const taskResults = $derived(flowTaskSearch.search(searchValue));
	const currentSearchScope = $derived(searchScopeFromPath(appNavigation.currentPath));
	const hasSuggestionResults = $derived(
		(currentSearchScope === 'mail' && mailMessageSearch.results.length > 0) ||
			(currentSearchScope === 'flow' && taskResults.length > 0) ||
			(currentSearchScope === 'calendar' && calendarResults.length > 0)
	);

	$effect(() => {
		mailMessageSearch.search(searchValue);
	});

	$effect(() => {
		if (!open) mailMessageSearch.reset();
	});

	$effect(() => {
		if (!open) return;
		attendanceClock.load();
		calendarEventSearch.load();
		flowTaskSearch.load();
	});

	function searchResultTimeLabel(result: CalendarSearchResult): string {
		return `${clockLabel(result.startDate)}-${clockLabel(result.endDate)}`;
	}

	function clockLabel(date: Date): string {
		return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
	}

	function searchScopeFromPath(pathname: string) {
		if (pathname.startsWith('/mail')) return 'mail';
		if (pathname.startsWith('/calendar')) return 'calendar';
		if (pathname.startsWith('/flow')) return 'flow';
		return '';
	}

	async function openFlowTask(task: FlowTask) {
		open = false;
		await goto(`/flow?task=${encodeURIComponent(task.id)}`);
	}

	function runClockIn(locationID: string) {
		open = false;
		attendanceClock.clock('clock_in', locationID);
	}

	function runClockOut() {
		open = false;
		attendanceClock.clock('clock_out', '');
	}

	function runLogOut() {
		open = false;
		confirmDelete({
			title: text.logOutConfirmTitle,
			description: text.logOutConfirmDescription,
			confirm: { text: text.logOut },
			cancel: { text: text.cancel },
			onConfirm: appNavigation.logOut
		});
	}

	async function openMailMessage(message: MailMessage) {
		open = false;
		await goto(`/mail/?mailbox=${encodeURIComponent(message.mailbox)}&uid=${message.uid}`);
	}

	async function openCalendarEvent(startDate: Date) {
		open = false;
		calendarNavigation.dateKey = dateKeyFromDate(startDate);
		if (!appNavigation.currentPath.startsWith('/calendar')) await goto('/calendar');
	}

	function locationShortcut(locationID: string) {
		const index = attendanceClock.locations.findIndex((location) => location.id === locationID);
		return locationShortcuts[index] ?? '';
	}

	function handleShortcut(event: KeyboardEvent) {
		if (searchValue !== '' || event.altKey || event.metaKey || event.ctrlKey) return;
		if (event.key === clockOutShortcut) {
			if (!attendanceClock.isClockedIn) return;
			event.preventDefault();
			runClockOut();
			return;
		}
		const location = attendanceClock.locations[locationShortcuts.indexOf(event.key)];
		if (!location) return;
		event.preventDefault();
		runClockIn(location.id);
	}
</script>

<Command.Dialog bind:open title={text.search} description={text.search} filter={koreanSearchScore} onkeydown={handleShortcut}>
	<Command.Input placeholder={text.search} bind:value={searchValue} />
	<Command.List>
		{#if !mailMessageSearch.results.length && !taskResults.length && !calendarResults.length}
			<Command.Empty>{text.searchNoResults}</Command.Empty>
		{/if}

		{#if !searchValue.trim() || hasSuggestionResults}
			<Command.Group forceMount heading={text.suggestions}>
				{#if !searchValue.trim()}
					{#if attendanceClock.isClockedIn}
						<Command.Item value="suggested-clock-out" keywords={[attendanceLabels.clockOut]} onSelect={runClockOut}>
							<LogOutIcon />
							{attendanceLabels.clockOut}
							<Command.Shortcut>{clockOutShortcut}</Command.Shortcut>
						</Command.Item>
					{:else if attendanceClock.defaultLocation}
						{@const defaultLocation = attendanceClock.defaultLocation}
						<Command.Item
							value="suggested-clock-in"
							keywords={[attendanceLabels.clockIn, defaultLocation.name]}
							onSelect={() => runClockIn(defaultLocation.id)}
						>
							<CircleIcon style="color: {defaultLocation.color}" />
							{attendanceLabels.clockIn} · {defaultLocation.name}
							<Command.Shortcut>{locationShortcut(defaultLocation.id)}</Command.Shortcut>
						</Command.Item>
					{/if}
				{:else if currentSearchScope === 'mail'}
					{@render mailResultItems()}
				{:else if currentSearchScope === 'flow'}
					{@render taskResultItems()}
				{:else if currentSearchScope === 'calendar'}
					{@render calendarResultItems()}
				{/if}
			</Command.Group>
		{/if}

		{#if currentSearchScope !== 'mail' && mailMessageSearch.results.length > 0}
			<Command.Separator />
			<Command.Group forceMount heading={text.mail}>
				{@render mailResultItems()}
			</Command.Group>
		{/if}

		{#if currentSearchScope !== 'flow' && taskResults.length > 0}
			<Command.Separator />
			<Command.Group forceMount heading={text.flow}>
				{@render taskResultItems()}
			</Command.Group>
		{/if}

		{#if currentSearchScope !== 'calendar' && calendarResults.length > 0}
			<Command.Separator />
			<Command.Group forceMount heading={text.calendar}>
				{@render calendarResultItems()}
			</Command.Group>
		{/if}

		<Command.Separator />

		<Command.Group heading={text.navigate}>
			{#each [...appNavigation.apps, ...appNavigation.workspace] as item (item.href)}
				{@const Icon = item.icon}
				<Command.LinkItem href={item.href} keywords={[item.label]} onSelect={() => (open = false)}>
					<Icon />
					{item.label}
				</Command.LinkItem>
			{/each}
		</Command.Group>

		<Command.Separator />

		<Command.Group heading={text.account}>
			{#each [appNavigation.contactItem] as item (item.href)}
				{@const Icon = item.icon}
				<Command.LinkItem href={item.href} target="_blank" rel="noopener noreferrer" onSelect={() => (open = false)}>
					<Icon />
					{item.label}
				</Command.LinkItem>
			{/each}
			<Command.Item keywords={[text.logOut]} onSelect={runLogOut}>
				<PowerOffIcon />
				{text.logOut}
			</Command.Item>
		</Command.Group>

		<Command.Separator />

		<Command.Group heading={text.attendance}>
			{#each attendanceClock.locations as location (location.id)}
				<Command.Item
					value="clock-in-{location.id}"
					keywords={[attendanceLabels.clockIn, location.name]}
					onSelect={() => runClockIn(location.id)}
				>
					<CircleIcon style="color: {location.color}" />
					{attendanceLabels.clockIn} · {location.name}
					<Command.Shortcut>{locationShortcut(location.id)}</Command.Shortcut>
				</Command.Item>
			{/each}
			<Command.Item
				value="clock-out"
				keywords={[attendanceLabels.clockOut]}
				disabled={!attendanceClock.isClockedIn}
				onSelect={runClockOut}
			>
				<LogOutIcon />
				{attendanceLabels.clockOut}
				<Command.Shortcut>{clockOutShortcut}</Command.Shortcut>
			</Command.Item>
		</Command.Group>
	</Command.List>
</Command.Dialog>

{#snippet mailResultItems()}
	{#each mailMessageSearch.results as message (`${message.mailbox}:${message.uid}`)}
		<Command.Item value="mail-message-{message.mailbox}-{message.uid}" forceMount class="p-1 [&>.cn-command-item-indicator]:hidden" onSelect={() => openMailMessage(message)}>
			<div class="min-w-0 flex-1">
				<MailMessageRow {message} isInteractive={false} text={mailLabels} />
			</div>
		</Command.Item>
	{/each}
{/snippet}

{#snippet taskResultItems()}
	{#each taskResults as task (task.id)}
		<Command.Item value="flow-task-{task.id}" forceMount class="p-1 [&>.cn-command-item-indicator]:hidden" onSelect={() => openFlowTask(task)}>
			<div class="min-w-0 flex-1">
				<FlowTaskBoardCard
					{task}
					businessFallback={flowLabels.task.businessFallback}
					openTask={() => openFlowTask(task)}
					isDraggable={false}
					isInteractive={false}
				/>
			</div>
		</Command.Item>
	{/each}
{/snippet}

{#snippet calendarResultItems()}
	{#each calendarResults as result (result.id)}
		<Command.Item
			value="calendar-event-{result.id}"
			forceMount
			class="[&>svg.cn-command-item-indicator]:hidden"
			onSelect={() => openCalendarEvent(result.startDate)}
		>
			<CalendarEventListCard
				class="min-w-0 flex-1"
				title={result.title}
				start={result.startDate}
				isAllDay={result.isAllDay}
				color={result.color}
				participants={result.participants}
				timeLabel={result.isAllDay ? '' : searchResultTimeLabel(result)}
				openEvent={() => openCalendarEvent(result.startDate)}
			/>
		</Command.Item>
	{/each}
{/snippet}

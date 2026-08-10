<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Tabs from '$lib/components/ui/tabs';
	import type { CRMAccount, CRMActivity, CRMContact, CRMOpportunity } from './crm-types';
	import { formatCRMDate, formatCRMDateTime, formatMoney, formatMoneyTotals, getActivitiesByAccount, getOpportunitiesByAccount, getProgressKind, getStageVariant, opportunityStageLabel } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = { account: CRMAccount; contacts: CRMContact[]; opportunities: CRMOpportunity[]; activities: CRMActivity[]; text: CRMText };
	let { account, contacts, opportunities, activities, text }: Props = $props();
	let accountContacts = $derived(contacts.filter((contact) => contact.accountID === account.id));
	let accountOpportunities = $derived(getOpportunitiesByAccount(account.id, opportunities));
	let accountActivities = $derived(getActivitiesByAccount(account.id, activities));
</script>

<div class="grid min-w-0 gap-4">
	<div class="grid gap-3 sm:grid-cols-2">
		<Card.Root><Card.Header class="pb-3"><Card.Description>{text.expectedValue}</Card.Description><Card.Title class="break-keep text-xl">{formatMoneyTotals(account.expectedValues, text.noValue)}</Card.Title></Card.Header></Card.Root>
		<Card.Root><Card.Header class="pb-3"><Card.Description>{text.openOpportunities}</Card.Description><Card.Title class="text-xl">{account.openOpportunityCount}</Card.Title></Card.Header></Card.Root>
	</div>

	<Tabs.Root value="overview" class="min-w-0 gap-4">
		<div class="max-w-full overflow-x-auto"><Tabs.List variant="line" class="w-max min-w-full justify-start"><Tabs.Trigger value="overview">{text.overview}</Tabs.Trigger><Tabs.Trigger value="contacts">{text.contacts}</Tabs.Trigger><Tabs.Trigger value="progress">{text.progress}</Tabs.Trigger><Tabs.Trigger value="activity">{text.activity}</Tabs.Trigger></Tabs.List></div>

		<Tabs.Content value="overview" class="grid gap-4">
			<Card.Root><Card.Header><Card.Title class="text-base">{text.owner}</Card.Title></Card.Header><Card.Content class="grid gap-2 text-sm"><p>{account.ownerName} · {account.team}</p><p class="break-all text-muted-foreground">{account.ownerEmail}</p>{#if account.address}<p class="text-muted-foreground">{account.address}</p>{/if}<div class="flex flex-wrap gap-1.5 pt-1">{#each account.tags as tag (tag)}<Badge variant="secondary">{tag}</Badge>{/each}</div></Card.Content></Card.Root>
		</Tabs.Content>

		<Tabs.Content value="contacts"><Card.Root><Card.Content class="grid gap-3 pt-5">{#each accountContacts as contact (contact.id)}<div class="rounded-md border p-3"><div class="flex flex-wrap items-center gap-2"><p class="font-medium">{contact.name}</p>{#if contact.isPrimary}<Badge variant="secondary">{text.primaryContact}</Badge>{/if}</div><p class="mt-1 break-all text-sm text-muted-foreground">{contact.title} · {contact.email}</p>{#if contact.phone}<p class="mt-1 text-sm text-muted-foreground">{contact.phone}</p>{/if}{#if contact.note}<p class="mt-2 text-sm leading-6 text-foreground/80">{contact.note}</p>{/if}</div>{/each}</Card.Content></Card.Root></Tabs.Content>

		<Tabs.Content value="progress"><Card.Root><Card.Content class="grid gap-3 pt-5">{#each accountOpportunities as opportunity (opportunity.id)}<div class="rounded-md border p-3"><div class="flex flex-wrap items-start justify-between gap-2"><div class="min-w-0"><p class="font-medium">{opportunity.name}</p><p class="mt-1 text-sm text-muted-foreground">{text.targetDate} {formatCRMDate(opportunity.targetDate)}</p></div><div class="flex flex-wrap gap-1.5"><Badge variant="outline">{text.progressKinds[getProgressKind(opportunity, account)]}</Badge><Badge variant={getStageVariant(opportunity.stage)}>{opportunityStageLabel(opportunity.stage, text)}</Badge></div></div>{#if opportunity.description}<p class="mt-3 text-sm leading-6 text-foreground/80">{opportunity.description}</p>{/if}<div class="mt-3 flex flex-wrap items-center justify-between gap-2"><p class="text-sm">{text.expectedValue} {formatMoney(opportunity.expectedValue, opportunity.currency, text.noValue)}</p></div></div>{:else}<p class="py-6 text-center text-sm text-muted-foreground">{text.noProgress}</p>{/each}</Card.Content></Card.Root></Tabs.Content>

		<Tabs.Content value="activity"><Card.Root><Card.Content class="grid gap-4 pt-5">{#each accountActivities as activity (activity.id)}<div class="grid grid-cols-[auto_minmax(0,1fr)] gap-3"><span class="mt-1 size-2 rounded-full bg-foreground"></span><div class="min-w-0 border-b pb-3 last:border-0 last:pb-0"><div class="flex flex-wrap items-center gap-2"><p class="text-sm font-medium">{activity.title}</p><Badge variant="outline">{text.activityKinds[activity.kind]}</Badge></div><p class="mt-1 text-xs text-muted-foreground">{formatCRMDateTime(activity.occurredAt)}</p><p class="mt-2 text-sm leading-6 text-foreground/80">{activity.summary}</p></div></div>{/each}</Card.Content></Card.Root></Tabs.Content>
	</Tabs.Root>
</div>

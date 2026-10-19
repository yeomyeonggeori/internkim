import { getContext, setContext } from 'svelte';
import type { CompanyActivity } from './activity';

type ActivityListContext = { people: CompanyActivity['people']; hiddenLabel: string };

const contextKey = Symbol('activity-list');

export function provideActivityListContext(context: () => ActivityListContext): void {
	setContext(contextKey, context);
}

export function activityListContext(): ActivityListContext {
	const read = getContext<() => ActivityListContext>(contextKey);
	return read();
}

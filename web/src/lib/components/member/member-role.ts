import type { Component } from 'svelte';
import CrownIcon from '@lucide/svelte/icons/crown';
import FlagTriangleRightIcon from '@lucide/svelte/icons/flag-triangle-right';

export type MemberRole = 'representative' | 'responsible';

export type MemberProfile = {
	memberID: string;
	externalID?: string;
	name: string;
	email: string;
	image?: string;
	jobTitle?: string;
	team?: string;
	phoneNumber?: string;
	roles: MemberRole[];
};

export const memberRoleLook: Record<MemberRole, { icon: Component; class: string }> = {
	representative: { icon: CrownIcon, class: 'bg-blue-500 text-white dark:bg-blue-600' },
	responsible: { icon: FlagTriangleRightIcon, class: 'bg-emerald-500 text-white dark:bg-emerald-600' }
};

export function memberRolesOf(memberID: string, representativeID = '', responsibleUserID = ''): MemberRole[] {
	const roles: MemberRole[] = [];
	if (representativeID && memberID === representativeID) roles.push('representative');
	if (responsibleUserID && memberID === responsibleUserID) roles.push('responsible');
	return roles;
}

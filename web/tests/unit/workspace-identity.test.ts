import { expect, test } from 'bun:test';
import { workspaceIdentityKey } from '../../src/lib/workspace-identity';
import type { SignedInMember } from '../../src/lib/supabase-session';
import type { WebAuthSession } from '../../src/lib/web-auth-session';

const session: WebAuthSession = { authenticated: true, email: 'sample@example.com', image: '', canViewTasks: true, cloudflareLoginURL: '', isUnavailable: false };
const member: SignedInMember = { memberID: 'member-a', companyID: 'company-a', role: 'admin', name: '이샘플', companySlug: 'example-co', companyLocale: 'ko' };
const scope = workspaceIdentityKey(session, member, 'https://project.example.com');

test('stable authority keeps content through locale, name, picture and ordinary revalidation changes', () => {
	expect(workspaceIdentityKey({ ...session, image: 'new-picture' }, { ...member, name: '박예시', companyLocale: 'en' }, 'https://project.example.com')).toBe(scope);
	expect(workspaceIdentityKey({ ...session }, { ...member }, 'https://project.example.com')).toBe(scope);
});

test('account, company, member, role, project and logout each retire routed state', () => {
	for (const changed of [
		workspaceIdentityKey({ ...session, email: 'other@example.com' }, member, 'https://project.example.com'),
		workspaceIdentityKey(session, { ...member, companyID: 'company-b' }, 'https://project.example.com'),
		workspaceIdentityKey(session, { ...member, memberID: 'member-b' }, 'https://project.example.com'),
		workspaceIdentityKey(session, { ...member, role: 'member' }, 'https://project.example.com'),
		workspaceIdentityKey(session, member, 'https://other-project.example.com'),
		workspaceIdentityKey({ ...session, authenticated: false, email: '' }),
		workspaceIdentityKey(null)
	]) expect(changed).not.toBe(scope);
});

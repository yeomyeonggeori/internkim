import { z } from 'zod';

import type { MessengerPlatform } from './protocol';

export const messengerIdentityCredentialKind = 'buzz-secret';

export const personalAccessTokenCredentialKind = 'api_key';

export const mailAccountCredentialKind = 'mail';

export const connectedAppCredentialKind = 'oauth_client';

export const messengerIdentityCredentialKinds = [messengerIdentityCredentialKind] as const;

export const memberCredentialKinds = [
  personalAccessTokenCredentialKind,
  ...messengerIdentityCredentialKinds,
  mailAccountCredentialKind,
] as const;

export const credentialKinds = memberCredentialKinds;

export const messengerIdentityCredentialKindSchema = z.enum(messengerIdentityCredentialKinds);

type MessengerIdentityCredentialKind = z.infer<typeof messengerIdentityCredentialKindSchema>;

const messengerPlatformOfIdentityCredentialKind: Record<MessengerIdentityCredentialKind, MessengerPlatform> = {
  [messengerIdentityCredentialKind]: 'buzz',
};

export function messengerPlatformOfCredentialKind(kind: string): MessengerPlatform | null {
  const identityKind = messengerIdentityCredentialKindSchema.safeParse(kind);
  return identityKind.success ? messengerPlatformOfIdentityCredentialKind[identityKind.data] : null;
}

export const memberCredentialKindSchema = z.enum(memberCredentialKinds);

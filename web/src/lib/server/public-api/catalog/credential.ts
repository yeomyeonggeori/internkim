import { z } from 'zod';

import { messengerPlatformNames } from './protocol';

export const messengerIdentityCredentialKind = 'buzz-secret';

export const personalAccessTokenCredentialKind = 'api_key';

export const mailAccountCredentialKind = 'mail';

export const connectedAppCredentialKind = 'oauth_client';

export const messengerIdentityCredentialKinds = [messengerIdentityCredentialKind] as const;

export const companyConnectionCredentialKinds = messengerPlatformNames;

export const memberCredentialKinds = [
  personalAccessTokenCredentialKind,
  ...messengerIdentityCredentialKinds,
  mailAccountCredentialKind,
] as const;

export const credentialKinds = [
  ...memberCredentialKinds,
  ...companyConnectionCredentialKinds,
] as const;

export const messengerIdentityCredentialKindSchema = z.enum(messengerIdentityCredentialKinds);

export const memberCredentialKindSchema = z.enum(memberCredentialKinds);

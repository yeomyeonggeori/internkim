# Push notifications

A member turns notifications on for one device, and the plane pushes to it.
Nothing is stored: there is no notification table, no inbox, no unread count.
The event a notification is about stays where it always was, so a missed push
costs only the interruption.

## What sends what

| Category | Sent when | Reaches |
|---|---|---|
| `message` | the relay sees a message addressed to someone | those addressed |
| `attendance` | a member clocks in or out | everyone else in the company |
| `leave` | a member asks for leave | the administrators, minus the asker |
| `task` | a task moves between statuses | the requester and the participants, minus whoever moved it |
| `calendar` | the hour a member chose arrives | that member, if they are on today's events |
| `approval` | an agent run needs a person | the requester |
| `mail` | new mail arrives | the account's owner |

The first five come from the central plane and need nothing else installed.
`approval` and `mail` come from `admind` on a device: the agent's runs and an
IMAP server are not things the plane can see.

Two switches decide whether a push is sent, and both belong to the person: the
category switch in their settings, and whether they have muted that
conversation. A member with no device subscribed is never reached at all, which
is the state everyone starts in.

## Keys the deployment needs

Web push is signed, so the Pages project carries a VAPID pair. Changing it
invalidates every subscription that exists — everyone has to turn notifications
on again — so it is set once.

```
VAPID_PUBLIC_KEY
VAPID_PRIVATE_KEY
VAPID_SUBJECT
```

Generate a pair with `bun run web/scripts/make-vapid-keys.ts`. Without them the
settings screen says so and every send answers `503`.

## The day digest needs a schedule

Everything else is sent by the request that caused it. The day digest has no
such request: it fires at an hour each member chose. `pg_cron` looks every
minute and `pg_net` calls the endpoint, reading both of these from the vault:

```sql
select vault.create_secret('https://<the app>', 'day_digest_app_url');
select vault.create_secret('<an agent api key>', 'day_digest_agent_key');
```

The agent key is one from the `agent` table, whose `api_key_hash` is the
SHA-256 of the key itself. With either secret missing the function returns and
nothing is sent, and no error is raised: a self-hosted install that has not
been set up yet should not fail once a minute.

Check that it is running:

```sql
select jobname, schedule from cron.job where jobname = 'announce-the-day';
select status_code, content from net._http_response order by created desc limit 5;
```

A response of `{"told":0,"reached":0}` at a minute nobody chose is the correct
answer.

## What the platforms do differently

| | Safari, any Apple device | Android Chrome | Desktop Chrome |
|---|---|---|---|
| Needs installing first | on iOS, to the home screen | no | no |
| Custom notification icon | ignored, the app icon is used | shown | shown |
| Its own entry in system settings | yes | when installed as a WebAPK | no, it is Chrome's |
| What the notification is labelled | the installed name | the installed name | Chrome, then the origin |

The icon a notification carries is honoured by Blink and dropped by WebKit, so
the same push shows a sender's face in Chrome and the application's own icon in
Safari on the same machine. Apple's implementation has no way to set one, which
is why the sender's name leads the body text.

An Android device that only shows notifications when the app is opened has put
Chrome to sleep; the push arrived and waited. Battery optimisation for Chrome
is what holds it, not the subscription.

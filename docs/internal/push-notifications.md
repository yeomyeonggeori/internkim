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
| `calendar` | an event's reminder lead time is reached | the members on that event |
| `approval` | an agent run needs a person | the requester |
| `mail` | new mail arrives | the account's owner |

The first five come from the central plane and need nothing else installed.
`approval` and `mail` come from `admind` on a device: the agent's runs and an
IMAP server are not things the plane can see.

Two switches decide whether a push is sent, and both belong to the person: the
category switch in their settings, and whether they have muted that
conversation. A member with no device subscribed is never reached at all, which
is the state everyone starts in.

The first message a conversation carries gives everyone in it a row in
`notification`, unmuted. Muting flips that row and unmuting flips it back, so
the table says who has ever been in a conversation and what each of them
chose, and a conversation nobody has spoken in has no rows to read.

## Keys the deployment needs

Web push is signed, so the plane carries a VAPID pair. Changing it invalidates
every subscription that exists, and everyone has to turn notifications on
again, so it is set once.

Founding a company sets it. `POST /api/company` posts to `setup-vapid` with the
founder's own token, the subject being the founder's address, and reports the
outcome as `notifications` in its answer. The call below is the same thing by
hand, for a deployment that was founded before this existed or whose founding
answered `failed`. It generates the pair and writes `vapid_public_key`,
`vapid_private_key` and `vapid_subject` into the vault:

```
curl -X POST https://<project>.supabase.co/functions/v1/setup-vapid \
  -H "Authorization: Bearer <the administrator's access token>" \
  -H 'Content-Type: application/json' \
  -d '{"subject":"mailto:ops@example.com"}'
```

A pair already in the vault is kept, and the answer is `{"stored":false}`. The
function never replaces a standing pair, because replacing it silently unsubscribes
every device that holds the old one. Replacing is a deliberate act with the
service key:

```sql
select public.vapid_keys_keep('<public>', '<private>', 'mailto:ops@example.com', true);
```

The edge senders read the three secrets from the vault and answer `503` while any
of them is missing. The web routes fall back to the deployment's own
`VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY` and `VAPID_SUBJECT`, which is what keeps a
Cloudflare deployment sending until its callers move across.

A deployment that is already sending with keys in its environment has devices
subscribed to that pair. Minting a fresh one would leave every send signed with a
key no device accepts, and the push services answer `403` where nothing prunes or
retries, so the function refuses with `409` while any web-push device stands. Move
such a deployment in by keeping the pair those devices already carry:

The values are the `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` the deployment is
already signing with, and the subject is its `VAPID_SUBJECT`:

```sql
select public.vapid_keys_keep('<VAPID_PUBLIC_KEY>', '<VAPID_PRIVATE_KEY>',
                              '<VAPID_SUBJECT>', true);
```

An operator who cannot recover the standing private key has no pair to adopt.
Clearing the subscriptions is then the honest move, because every device holding
that key is already unreachable:

```sql
delete from public.push_device where kind = 'web-push';
```

Everyone turns notifications on again after that, and the browser re-subscribes
because it checks which key its subscription was made with.

## The reminder needs a schedule

Everything else is sent by the request that caused it. A calendar reminder has
no such request: it fires the number of minutes before an event that whoever
made the event chose, held in `task.notify_minutes_before`. An event that names
no lead time reminds nobody. `pg_cron` looks every minute and `pg_net` posts to
`announce-event-reminder`, reading the address and the key it carries from the
vault.

Founding a company sets both, through the same `POST /api/company` call that
sets the VAPID pair. By hand, an administrator posts to `setup-digest-key`:

```
curl -X POST https://<project>.supabase.co/functions/v1/setup-digest-key \
  -H "Authorization: Bearer <the administrator's access token>"
```

The call issues the agent key, keeps it in `day_digest_agent_key`, and records
its SHA-256 as a `day digest` row in the `agent` table. One key serves the
project, so a call made while a valid one stands answers `{"stored":false}` and
leaves it alone. The address goes to `project_url`, taken from the function's
own `SUPABASE_URL` and never from anything the caller sends.

With no address and no key the function returns and nothing is sent, and no
error is raised: a self-hosted install that has not been set up yet should not
fail once a minute.

Check that it is running:

```sql
select jobname, schedule from cron.job where jobname = 'announce-event-reminders';
select status_code, content from net._http_response order by created desc limit 5;
```

A minute in which no event is due sends nothing, which is the correct answer.

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

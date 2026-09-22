# Where the archive signing key lives

[`native-packaging.md`](./native-packaging.md) §6 says the private half of the
apt repository's signing key "lives in the release CI secret store". There is
no CI in this repository and releases are cut by hand, so that store does not
exist. This is what the repository can actually offer, and what it costs.

Nothing here has been decided and no key has been generated. Everything built
so far runs against a throwaway key whose user ID says so.

## What the key is

Whoever holds it can publish a package that every company host installs as
root on the next `apt upgrade`, without touching R2, Cloudflare, or any
account we control. Apt checks one thing before it runs a maintainer script:
that the `Release` was signed by a key the machine was told to trust. The key
is a larger credential than the R2 write token, which only reaches a bucket
whose contents are still refused without a matching signature.

It is also the credential with the worst recovery story. Rotating it means
reaching every installed machine to replace
`/usr/share/keyrings/internkim-archive-keyring.pgp`, and the channel for
reaching them is apt, which the old key is what authenticates. A key that
leaks is a key that stays leaked until someone visits each machine.

## What the repository does with secrets today

Three shapes exist, and a credential has exactly one home —
`internal/cli/one_home_for_each_credential_test.go` fails a credential that
can be read from two places, so "either works" is not available.

| Shape | Example | Read by |
|---|---|---|
| An environment value | `INTERNKIM_RELEASE_DOWNLOAD_TOKEN` | a shell driven by hand |
| A path to a mode-0600 file | `ADMIN_ASSERTION_KEY_PATH` | a server, so the key never enters `ps eww` |
| A mode-0600 file at a fixed path | `.local/secrets/…` | one hardcoded constant |

`INTERNKIM_RELEASE_SIGNING_KEY`, which signs the device release manifest, is
the nearest neighbour and is not a precedent: it is an HMAC secret with no
public half, so it answers neither how a verification key is distributed nor
what happens when one is withdrawn.

The archive key takes the second shape. `INTERNKIM_APT_SIGNING_KEY_PATH` names
an exported OpenPGP secret key, `internkim release apt` imports it into a gpg
homedir it creates and destroys around the run, and the key's bytes never
enter an environment variable or the process table. That much is built and is
independent of the decision below, because every option here is a different
answer to what that path points at.

## The options

**(a) A file under `.local/secrets/`.** One `chmod 600` file on whoever cuts
releases. It costs nothing to build — it is the default the code already
falls back to — and `.local/` is gitignored, so it cannot be committed by
accident. It is also a plain file on a laptop: any process running as that
user can read it, a backup tool can copy it, and a stolen machine is a stolen
key. There is no record of when it was used.

**(b) The OS keychain, through the `monkeys` helper.** The key stays in
macOS's keychain and is handed to the command at the moment it runs. The
laptop's own unlock protects it, and a backup does not carry it. It is one
person's laptop either way, and the key still exists in plaintext in that
process's memory and in the gpg homedir for the length of the run. Nothing in
this repository uses `monkeys` yet, so this is the option that adds a
dependency.

**(c) A hardware token.** A YubiKey or equivalent holding the private key,
which signs without ever releasing it. Extraction stops being possible,
signing requires physical presence, and a lost token is revoked rather than
assumed copied. `internkim release apt` shells out to `gpg` specifically so
this costs nothing to adopt: a smartcard-backed key is the same `gpg
--clearsign` call, and `INTERNKIM_APT_SIGNING_KEY_PATH` points at the public
stub. The cost is real money, a device that must be present to cut a release,
and a second token kept somewhere safe, because one token is one hardware
failure away from the rotation problem above.

**(d) Wait, and sign nothing.** Publish no repository until the key is
decided. This is where things stand today and it is not free: `install.sh`
still has no Debian branch, so the packaged path is not reachable by a
customer either way, but every week the decision waits is a week the rig
proves the mechanism against a throwaway key rather than the real one.

## Recommendation

**(c), with (a) as the deliberate interim.**

The asymmetry decides it. The key's blast radius is root on every customer
machine and its rotation story is a site visit, which is the profile hardware
exists for; the difference in what it costs to use is one device in a drawer.
Option (b) is a smaller version of the same protection and inherits the same
extraction risk, so it is not worth adding a dependency for.

The interim matters because (c) cannot be adopted the day it is chosen: the
token has to arrive, and the key has to be generated on it rather than
generated and copied to it, or the copy is what leaked. Until then a file
under `.local/secrets/` on one machine, used to publish only the `testing`
suite, is enough to keep building. The `stable` suite waits for the hardware.

Two things should be settled at the same time as the key, because they are
much cheaper before the first customer than after:

- **The public half is committed to this repository** and shipped as
  `/usr/share/keyrings/internkim-archive-keyring.pgp`. It is also published at
  `deb/internkim-archive-keyring.pgp`, which is convenient and is not
  trust — a keyring fetched over the same connection as the packages proves
  nothing. The committed copy is what `install.sh` should carry.
- **An expiry and a successor.** A key with no expiry is a key nobody ever
  rotates. Generating the successor at the same time as the key, signed by it
  and kept offline, turns rotation from a site visit into a `Signed-By:` that
  already names both.

## What is built, and against what

`internkim release apt` and `tools/test-apt-repository` work today against a
key generated per run, with the user ID `InternKim Install Rig TEST KEY
<rig@invalid.internkim.test>`, held in a homedir under `/tmp` that is removed
when the run ends. It is not in the repository, it is not reused between runs,
and nothing published from it has left this machine.

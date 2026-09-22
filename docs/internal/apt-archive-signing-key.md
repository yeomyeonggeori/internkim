# Where the archive signing key lives

[`native-packaging.md`](./native-packaging.md) §6 says the private half of the
apt repository's signing key "lives in the release CI secret store". There is
no CI in this repository and releases are cut by hand, so that store does not
exist. This is what the repository can actually offer, and what it costs.

The decision below was taken and the key exists. What it is, and what replacing
it costs, is the last section; the rest of this document is the reasoning that
chose it, kept because the hardware-token step has not been taken yet.

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

The archive key takes the second shape at the point it meets gpg.
`internkim release apt` resolves the key to a mode-0600 file, imports that into
a gpg homedir it creates and destroys around the run, and removes the file
afterwards. That much is built and is independent of the decision below,
because every option here is a different answer to where the bytes come from
before that file is written.

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

**(c), with (b) as the deliberate interim.**

The asymmetry decides the destination. The key's blast radius is root on every
customer machine and its rotation story is a site visit, which is the profile
hardware exists for; the difference in what it costs to use is one device in a
drawer.

The interim was first written as (a), on the argument that (b) protects against
the same extraction and was not worth a dependency for. The dependency turned
out to be already installed: `monkeys` is on this machine, its manifest commits
key *names* rather than values, and the vault is somewhere a loose file is not.
Against a laptop that is lost or a backup that is too broad, which is what
actually happens to an interim, the vault wins, and it costs one word on a
command line. The argument against (b) was right about what it protects against
and wrong about what it cost.

Neither interim closes the gap (c) exists for. A key the signing machine can
read is a key malware running as that user can read, whichever store holds it.
That is why the interim publishes only the `testing` suite; the `stable` suite
waits for the hardware.

`internkim release apt` therefore takes the key from `INTERNKIM_APT_SIGNING_KEY`
in the environment `monkeys run` provides, and immediately stops it being an
environment value: it writes the key to a mode-0600 file in a private
directory, unsets the variable so nothing it starts inherits it, hands gpg the
path, and removes the file when the run ends. This repository's convention is
that secrets reach programs as paths, because an environment value is readable
from the process table by anything running as that user. A signing run lasts
seconds rather than a daemon's lifetime, so the exposure is smaller, but the
shape stays the same as everything else here. That function is also the seam
(c) replaces: a token changes what it hands gpg, and nothing above it.

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

## The key that exists

```
InternKim Archive Signing Key <support@intern.kim>
RSA 4096, sign-only, no passphrase
BC8C 0D89 911E 0A78 A184 14F1 7351 8E28 F70E 2655
created 2026-09-22, expires 2029-09-21
```

It lives in the vault as `INTERNKIM_APT_SIGNING_KEY` and nowhere else. The
homedir it was generated in was destroyed, and with it the revocation
certificate gpg wrote there; `gpg --gen-revoke` makes another from the secret
key whenever one is wanted, so what was discarded is a convenience rather than
the ability to revoke. The public half is published at
`deb/internkim-archive-keyring.pgp` and is not committed to this repository, so
the second of the two things this document says to settle with the key is still
open.

The address is the one the package already carries as its `Maintainer` rather
than a new one, because an archive key's user ID is read by a person deciding
whether to trust what it signed, and a second address would be one more thing
to recognise.

It signs `trixie-testing`, which is the only suite published. `trixie-stable`
is what an install follows and nothing has been published to it, which is the
interim this document argues for: the key a laptop can read signs the suite no
customer machine looks at.

**Expiry.** Three years was chosen over none. A key with no expiry is a key
nobody rotates, and today the cost of an expiry is nothing, because no machine
has the keyring written down. It will not be nothing later: on the day it
lapses, every installed machine refuses updates until its
`/usr/share/keyrings/internkim-archive-keyring.pgp` is replaced, and apt is the
channel that would have carried the replacement. What turns that from a site
visit into a `Signed-By` naming two keys is a successor generated now, signed
by this one and kept offline. That has not been done.

**Replacing it.** Generate the new key, `monkeys remember
INTERNKIM_APT_SIGNING_KEY` over the old value, and republish every suite —
`internkim release apt` exports the public half beside the repository from
whatever key it just signed with, so one publish moves both halves together.
That is the whole procedure while no machine trusts the old key. Once one does,
the same two commands leave it unable to update, because the keyring it holds
is the old key and the only thing that would deliver the new one is the
repository the old key no longer signs. Nothing in this repository closes that
gap today.

## What is built, and against what

`tools/test-apt-repository` runs against a key generated per run, with the user
ID `InternKim Install Rig TEST KEY <rig@invalid.internkim.test>`, held in a
homedir under `/tmp` that is removed when the run ends. It is not in the
repository and is not reused between runs.
`tools/test-published-apt-repository` is the one that reaches the archive key,
because it installs from what was published rather than from what it built.

The vault path is exercised rather than assumed.
`tools/test-apt-repository --through-the-vault` puts that same throwaway key
into the vault under the real name, publishes with the real invocation, and
verifies the signature on what came out:

```
$ tools/test-apt-repository --through-the-vault
stored a throwaway key in the vault as INTERNKIM_APT_SIGNING_KEY
signed suite trixie-testing with 95AD6BE3A47385DD583A30E36AC70C0EF66B374B
wrote 9 objects to /var/folders/…/apt-vault-hi4_0ium/release
gpg: Good signature from "InternKim Install Rig TEST KEY <rig@invalid.internkim.test>" [ultimate]
removed the throwaway key from the vault
```

It uses the real name because a rehearsal under a different name would not
exercise what a person runs. `monkeys remember` overwrites, so it refuses to
start when that name already holds a key, and it forgets what it stored on
every path out including a failure. Now that the name holds the archive key it
refuses on the first check, which is what that check is for: it will not run
again until the archive key is taken out of the vault, and taking it out to run
a rig is not a thing to do.

Two invariants are tests rather than prose.
`TestTheArchiveSigningKeyComesFromTheVaultAlone` fails if a file under
`.local/secrets/` can supply the key, which is how option (a) would come back
as a fallback and give the key two homes;
`TestMaterialisingTheKeyTakesItOutOfTheEnvironment` fails if the value is still
in the environment after the file is written, which is what a child process
would otherwise inherit. There is no `--signing-key` flag, and
`test_no_flag_offers_the_signing_key_a_second_home` fails if one comes back.

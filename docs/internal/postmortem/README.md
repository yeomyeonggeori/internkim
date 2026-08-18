# Postmortems

An incident story lives here and nowhere else.

`AGENTS.md` carries standing orders: the rule an agent needs in context every
session, and the command that gets a device out of trouble. It is read in full
at the start of every session, so a paragraph of narrative there is paid for
thousands of times. The narrative still matters — a rule whose cost nobody
remembers is a rule someone argues away — so it moves here and the rule links to
it.

A postmortem is chronology, not a teaching sequence. It records what was
believed, what was observed, and what the evidence turned out to be. Write it
when an incident took real time to diagnose or when the failure looked like
health: a green `systemctl`, a deploy that reported success, a check that passed
because it was reading the wrong file.

Name a file `NNNN-what-actually-broke.md`, numbered in the order they are
written. The title names the mechanism, not the component: a reader scanning the
directory should be able to tell whether their symptom is in here.

Each one carries, in this order: what the symptom looked like, what was actually
wrong, why the usual recovery did not apply, and what would have caught it
earlier. The last section is the one that pays for the document, because it is
what turns into a rule, a test, or a gate.

Do not put a runbook here. A procedure someone follows belongs next to the thing
it operates, and this file links to it instead.

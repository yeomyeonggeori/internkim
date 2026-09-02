# The device path

On 2026-09-02 the device path was frozen. Jetson, OTA releases, the
cloud-hypervisor guest and vsock keep working and keep getting bug fixes. No new
design is implemented against them: a feature that would need one of them
designed differently is a feature that belongs on the central plane instead.

Mattermost is not part of the freeze. It is being removed, so a rule here that
names it describes what still runs today and not what to build on.

The documents below hold the rules `AGENTS.md` used to carry. They apply when
you are working on the device path; nothing in them constrains the plane.

| Document | Covers |
|---|---|
| [deploying-a-device.md](./deploying-a-device.md) | OTA releases, `setup`, the two-deploy rule, the local LLM |
| [the-local-fleet.md](./the-local-fleet.md) | The disposable fleet VM, its scenarios, reprovision |
| [bringing-device-data-across.md](./bringing-device-data-across.md) | Moving a device's records onto the plane |
| [the-companion-runtime.md](./the-companion-runtime.md) | The user's local trusted runtime |

Hardware setup and support for the Jetson itself is
[../jetson-setup-and-support.md](../jetson-setup-and-support.md), and a device
that cannot be reached is [../runbook-device-unreachable.md](../runbook-device-unreachable.md).

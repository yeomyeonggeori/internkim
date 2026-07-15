# Expensive Tests

Expensive tests make paid model calls. Each JSON file is one ordered scenario.
Its steps share one virtual session, stop at the first failure, and pass only
when every step satisfies its strict assertions.

```bash
./internkim test expensive
./internkim test expensive --scenario task-lifecycle
./internkim test expensive --scenario document-lifecycle
./internkim test expensive --maximum-model-tier high
./internkim test expensive --real
./internkim test full
```

The default ceiling is `xlow`; image input may use `low`. With a ceiling,
coding work uses the ceiling tier. Without a ceiling, `--real` uses the normal
configured coding model and production tier routing. The default seed is `41`
and temperature is `0`. Calendar and document lifecycle scenarios use short,
observable capability and file-tool flows designed for the default `xlow`
ceiling.

`cheap` runs only non-paid checks. `expensive` does not include `cheap`.
`full` runs `cheap` first and then every expensive scenario.

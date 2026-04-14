# parse-array-info

Read an `array-info.yaml` file and emit `export` statements for the selected
platforms and features. The E2E test runner (`run-e2e-test.sh`) calls this tool
with `eval` so that the exported variables are available to the rest of the
test session.

## Prerequisites

- Go 1.21+

## How it works

`array-info.yaml` is a flat YAML file whose top-level keys are **section
names**. Each section contains environment-variable key/value pairs.

Section naming convention:

| Section name | Loaded when |
|---|---|
| `global` | Always (namespace prefix, shared settings) |
| `powerflex` | Platform `powerflex` is active |
| `powerflex-auth` | Platform `powerflex` **and** feature `auth` are both active |
| `auth-common` | Standalone feature — loaded when `auth-common` is in `-features` |

Within each loaded section, every non-empty value is printed as a shell
`export` statement. Empty values are silently skipped.

## Running

```
go run main.go -platforms <list> -features <list> -file <path>
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-platforms` | `""` | Comma-separated active platforms (e.g. `powerflex,powerstore`) |
| `-features` | `""` | Comma-separated active features (e.g. `auth,zoning,replication,oidc,sftp,auth-common`) |
| `-file` | `array-info.yaml` | Path to the YAML configuration file |

### Platforms

`powerflex`, `powerscale`, `powermax`, `powerstore`, `unity`, `cosi`

### Features

`auth`, `zoning`, `replication`, `oidc`, `sftp`, `auth-common`

## Examples

### Load everything for PowerStore (base + auth)

```
$ go run main.go -platforms powerstore -features auth,auth-common -file ../../array-info.yaml
export NS_PREFIX='e2e'
export REDIS_USER='user'
export REDIS_PASS='pass'
export POWERSTORE_USER='username'
export POWERSTORE_PASS='password'
export POWERSTORE_GLOBALID='myglobalpowerstoreid'
export POWERSTORE_ENDPOINT='1.1.1.1'
export POWERSTORE_PROTOCOL='auto'
export POWERSTORE_AUTH_ENDPOINT='127.0.0.1:9400'
export POWERSTORE_STORAGE='powerstore'
...
```

### Load only base platform sections (no features)

```
$ go run main.go -platforms powerflex,powerscale -features "" -file ../../array-info.yaml
export NS_PREFIX='e2e'
export POWERFLEX_USER='admin'
export POWERFLEX_PASS='Password123!'
...
export POWERSCALE_CLUSTER='Isilon-System-Name'
export POWERSCALE_USER='root'
...
```

### Usage inside run-e2e-test.sh

The test runner builds the platform and feature lists from its CLI flags and
then evals the output:

```bash
cd ./scripts/parse-array-info
eval "$(go run main.go \
  -platforms "$platforms" \
  -features "$features" \
  -file "../../$ARRAY_INFO_FILE")"
cd ../..
```

After `eval`, every non-empty variable from the matching sections is available
as a regular shell export for the remainder of the test run.

## Configuration file reference

See [`array-info.yaml.sample`](../../array-info.yaml.sample) for the full list
of sections and variables with example values.

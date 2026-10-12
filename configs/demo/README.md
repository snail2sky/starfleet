# Starfleet demo configs

A runnable example covering the three files you normally write:

| File | What it shows |
|---|---|
| `starfleet.yml` | main config: listener, **auth (WebUI login)**, logging |
| `conf.d/demo-web.yml` | a managed **process** with logs, labels, restart policy |
| `conf.d/demo-backup.yml` | a **schedule** (cron job) with timeout and enable flag |

## Run it

```bash
starfleet serve -c starfleet.yml
```

Then, in another terminal:

```bash
starfleet -c starfleet.yml status          # process table
starfleet -c starfleet.yml schedules       # cron jobs
starfleet -c starfleet.yml logs -f         # starfleet's own log
```

Open <http://localhost:8080> and log in with `admin` / `demo-password`.
On the Processes page click a row for details, try the Start/Stop buttons,
and click the Autorestart cell to cycle the restart policy — every change
is written back into the config file.

Watch `demo-backup` fire every 2 minutes on the Schedules page (Run now
button triggers it immediately; History shows past runs).

For the complete field reference see `docs/configuration.md` in the
repository (or the commented `examples/starfleet.yml` in the release).

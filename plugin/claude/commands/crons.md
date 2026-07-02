---
title: Stardust maintenance and digest crons
description: Arm the stardust maintenance and digest crons as native Claude Code scheduled tasks, in local time.
argument-hint: ""
allowed-tools: Bash, Read, CronCreate, CronList
---

You are arming two native Claude Code crons for the stardust-plugin. They run only because
the user invoked this command. Never create crons at install time and never create them
without being asked. Keep each cron prompt terse: it runs fixed commands and stops.

## Step 1: ground the clock and the workspace

1. Run `date` to read the current local time and timezone. Do not trust any remembered
   time. Compute every schedule in local time, since native crons take a five-field vixie
   schedule in local time.
2. Resolve the workspace root and mode by running
   `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and reading the `MODE` and `ROOT`
   lines. If `MODE` is `none`, tell the user there is no workspace to maintain and stop.
3. Read tunables from `${CLAUDE_PLUGIN_DATA}/config.json` with `jq`: `maintenanceCron`
   (default `0 */2 * * *`) and `digestHourLocal` (default `8`). The digest schedule is
   `30 <digestHourLocal> * * *`.

## Step 2: create the maintenance cron

Create a recurring cron with `CronCreate` on the `maintenanceCron` schedule. Its prompt:

> Run, in this order, from the stardust workspace root: `stardust index --background`, then
> `stardust registry`, then `stardust sync`. Report one line with the result. Then self-heal
> the plugin crons: call `CronList`, and for any stardust-plugin cron that is missing or
> within one day of its 7-day expiry, re-create it with `CronCreate` on its original
> schedule. Stop after one line.

## Step 3: create the digest cron

Create a recurring cron with `CronCreate` on the `30 <digestHourLocal> * * *` schedule, in
the local morning. Its prompt:

> From the stardust workspace root, run `stardust digest --advance`. Summarize the digest in
> a few lines grouped by area, surfacing open commitments. Deliver by printing the summary;
> if a notification sink is configured, also send it. Stop after the summary.

## Step 4: confirm and document the expiry

1. Call `CronList` and confirm both jobs appear with the expected local-time schedules.
2. Tell the user the residual: native recurring crons auto-expire after 7 days. The
   maintenance cron self-heals as long as a session runs at least weekly. If no session runs
   for over 7 days, re-run `/stardust:crons` to re-arm both.
3. Native crons cap at 50 per session. If `CronCreate` reports the cap, say so plainly and
   stop; the commit hook still keeps repo mode fresh in the meantime.

Schedules and the digest hour come from `config.json`, so both can be tuned there without
editing this command.

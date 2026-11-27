# Google Tasks API v1: what it holds and how it reports changes

Researched 2026-09-29 for issue #95, part of map #92. It answers what a two-way
sync between a Repo's `tasks.md` and Google Tasks can carry, how the server
learns what changed on Google's side, what it costs in quota, and how one
person authorizes a LAN server.

Each fact is tagged:

- **[docs]**: verified from a Google primary source, linked in the section.
- **[inferred]**: reasoned from the docs, from gtasks-md's code, or from a
  community report, and not stated by Google. Treat it as a hypothesis to test
  against a real account.

Sources:

- Task resource: <https://developers.google.com/workspace/tasks/reference/rest/v1/tasks>
- `tasks.list`: <https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/list>
- `tasks.insert`: <https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/insert>
- `tasks.move`: <https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/move>
- TaskList resource and `tasklists.list`: <https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists>
- Parameter table: <https://developers.google.com/workspace/tasks/params>
- REST index and discovery document (revision `20260927`): <https://developers.google.com/workspace/tasks/reference/rest>, `https://tasks.googleapis.com/$discovery/rest?version=v1`
- Quotas: <https://developers.google.com/workspace/tasks/limits>
- Scopes: <https://developers.google.com/workspace/tasks/auth>
- OAuth overview (refresh-token expiry): <https://developers.google.com/identity/protocols/oauth2>
- OAuth for installed apps: <https://developers.google.com/identity/protocols/oauth2/native-app>
- OAuth for limited-input devices: <https://developers.google.com/identity/protocols/oauth2/limited-input-device>
- Publishing status: <https://support.google.com/cloud/answer/15549945>
- Unverified apps: <https://support.google.com/cloud/answer/7454865>
- gtasks-md: the repo-local copy in `tmp/wayfinder/src/gt_*.py`, and section 1 of `docs/research/tasks-md-sources.md`

---

## 1. What a Task holds

The Task resource has these fields. Everything is a string unless noted. **[docs]**

| Field | Writable | What it holds |
| --- | --- | --- |
| `id` | set by server | Task identifier. |
| `etag` | no | ETag of the resource. |
| `title` | yes | Max **1024** characters. |
| `notes` | yes | Max **8192** characters. Optional. Tasks assigned from Google Docs cannot have notes. |
| `status` | yes | Exactly two values: `needsAction` or `completed`. |
| `due` | yes | RFC 3339 timestamp, but **only the date is recorded**. "The time portion of the timestamp is discarded when setting this field. It isn't possible to read or write the time that a task is scheduled for using the API." It is the day the task should be done, and Google says it is **not** a deadline. |
| `completed` | yes | RFC 3339 completion timestamp. Omitted while not completed. |
| `updated` | no | RFC 3339 last-modification time. |
| `parent` | no | Parent task id. Omitted for a top-level task. It is changed only through `tasks.move` (or `parent` on `tasks.insert`). |
| `position` | no | Opaque string. Siblings sort **lexicographically** by it. It is changed only through `move` (or `previous` on `insert`). |
| `deleted` | yes (bool) | Soft-delete flag. It defaults to false. |
| `hidden` | no (bool) | True when the task was completed at the time the list was last **cleared** (`tasks.clear`). |
| `links[]` | **no** | `{type, description, link}`, where type is for example `email`, `generic`, `chat_message` or `keep_note`. It is read-only and output-only, so the API cannot attach a URL to a task. |
| `webViewLink` | no | A link to the task in the Google Tasks web UI. |
| `assignmentInfo` | no | Set for tasks assigned to you from Docs or Chat spaces (`DOCUMENT`/`SPACE`, and `GMAIL` in the enum). |
| `selfLink`, `kind` | no | Plumbing. |

**Fields that do not exist** in the discovery document's Task schema: star or
priority, recurrence rule, tags or labels, color, author or last editor, a
start date, and a time of day. **[docs]** (checked against the schema's
property list)

The Google Tasks app has **repeating tasks**. The API only mentions them
through restrictions: they "cannot be set as parent tasks … or be moved under a
parent task," and they "cannot currently be moved between lists." No field
exposes the rule. **[docs]** From that, a repeating task most likely comes
through the API as a plain task with one `due`, and its recurrence is invisible
and cannot be written. **[inferred]**

A TaskList holds only `id`, `etag`, `title` (max 1024), `updated`, and
`selfLink`. It has no color, no ordering field and no description. **[docs]**

### Nesting

- A subtask is expressed by `parent`. Up to **2,000 subtasks per task**. **[docs]** (`tasks.move`)
- A parent must exist in the same list and **cannot be hidden**. Assigned
  tasks and repeating tasks can be neither parents nor subtasks. **[docs]**
- "Tasks that are both completed and hidden **cannot be nested**, so the parent
  field must be empty", and they "can only be moved to position 0." **[docs]**
  So clearing a list likely detaches completed subtasks from their parents in
  what the API reports. **[inferred]**
- **Depth is one level.** A subtask cannot have subtasks. The reference never
  states a depth limit. This comes from gtasks-md's design ("Google Tasks
  allows only one level") and from how the Google Tasks apps behave. **[inferred]**
  A test insert with `parent` set to a subtask would settle it.

### Limits **[docs]**

- 1024 characters for a title, whether task or list. 8192 characters for notes.
- **2,000 task lists** per user (`tasklists.list`).
- **20,000 non-hidden tasks per list**, and **100,000 tasks in total** per user
  (`tasks.list`, `tasks.insert`).
- 2,000 subtasks per task.
- Page size: `tasks.list` defaults to 20 and allows up to 100.
  `tasklists.list` defaults to 1000 and allows up to 1000. Note that the older
  parameter table still says 100 for lists.

---

## 2. How it reports changes

The **only** change-detection mechanism is polling `tasks.list` with
`updatedMin`. **[docs]**

- **No push.** The REST index and the discovery document list only
  `delete/get/insert/list/patch/update` on tasklists and
  `clear/delete/get/insert/list/move/patch/update` on tasks. There is no
  `watch` method, no channels and no webhooks. **[docs]**
- **No sync tokens.** `tasks.list` has no `syncToken` parameter and returns no
  `nextSyncToken`, unlike Calendar or Drive. **[docs]**
- **`updatedMin`** (RFC 3339) filters to tasks whose `updated` is at or after
  the bound. **[docs]** To see deletions and cleared completions, a poll must
  also pass `showDeleted=true`, `showHidden=true` and `showCompleted=true`. A
  deleted task then comes back as a tombstone with `deleted: true`. **[docs]**
  (the flags) / **[inferred]** (that the tombstone carries a fresh `updated`)
- **How long tombstones are kept is undocumented.** A server that is offline
  longer than that retention would miss deletions and would need a full
  re-list plus a diff by id to find them. **[inferred]**
- **Default conflict.** The reference says `showHidden` defaults to **false**.
  The older parameter table says **true**. Always pass it explicitly. **[docs]**
- `showCompleted` alone does not return tasks completed in Google's own apps:
  "showHidden must also be True to show tasks completed in first party
  clients." **[docs]** gtasks-md hit this and passes `showHidden=completed`.
- **`TaskList.updated` cannot be used to skip unchanged lists.** One
  StackOverflow report (2022) found that it moves on status changes, deletes
  and reorders, but not on title, notes or due edits. **[inferred]** So every
  poll lists tasks in every mapped list.
- **Assigned tasks** (from Docs or Chat) are left out unless
  `showAssigned=true`. **[docs]**
- **ETags.** Every Task, TaskList and list response carries an `etag`.
  **[docs]** Google does **not** document that `If-Match` on
  `patch`/`update`/`delete` is honoured with `412 Precondition Failed`, or that
  `If-None-Match` on `get`/`list` returns `304`. **[inferred / untested]** Until
  that is tested, a stale-write guard toward Google has to be client-side:
  re-`get` the task and compare `updated`/`etag` just before patching. That is
  still a race, just a narrow one. **[inferred]**
- **Partial updates:** `PATCH` sends only the changed fields, and the `fields`
  parameter trims responses. **[docs]** (<https://developers.google.com/workspace/tasks/performance>)
- **Batching:** gtasks-md sends one batched HTTP request per list for
  insert/patch/delete, and sends `move` calls **sequentially**, because
  concurrent moves cannot name the same `previous`. **[inferred from gtasks-md
  code]** Each call in a batch is assumed to count as one query against quota.
  **[inferred]**

---

## 3. Quotas

- **50,000 queries per day** is the courtesy limit per Cloud project. A quota
  increase can be requested, and approval is not guaranteed. **[docs]**
- Per-minute and per-user limits are not stated on the Tasks limits page.
  They show in the Cloud console's Quotas page for the project. **[inferred]**
- **Polling budget** (arithmetic, one account): each poll costs one
  `tasklists.list` plus one `tasks.list` per mapped list, and more if a list
  pages past 100 changed tasks.
  - 10 Repos polled every 60 s: 11 × 1,440 = **15,840/day**.
  - 10 Repos every 30 s: **31,680/day**.
  - 20 Repos every 30 s: 21 × 2,880 = **60,480/day**, which is over the limit.

  Writes come on top of these numbers. A poll interval of 60 s or more, or
  polling only lists touched since the last cycle (which the `TaskList.updated`
  gap above makes unsafe), keeps a personal tracker well inside the limit.
  **[inferred]**

---

## 4. OAuth for one person on a LAN server

- **Scopes:** `https://www.googleapis.com/auth/tasks` for read and write, and
  `…/tasks.readonly`. **[docs]** Google's scope page does not label the scope
  sensitive or restricted. It is widely treated as sensitive, so an unverified
  production app shows the "unverified app" screen. **[inferred]**
- **Testing mode tokens expire in 7 days.** "A Google Cloud Platform project
  with an OAuth consent screen configured for an external user type and a
  publishing status of 'Testing' is issued a refresh token expiring in 7 days."
  The only exception is a request for nothing beyond
  `openid`/`email`/`profile`, and the Tasks scope is not covered. **[docs]**
  Testing also caps the app at 100 listed test users and shows a warning
  before consent. **[docs]**
- **In Production, unverified:** the 7-day rule is stated only for Testing. A
  production app that has not been verified shows the unverified-app screen,
  which the person clicks through, and is capped at **100 new users**.
  **[docs]** For one person that cap does not matter, and refresh tokens then
  follow the ordinary rules. **[inferred]** Verification is not needed for
  "apps in development … unless you decide to launch it to the public." **[docs]**
- **Ordinary refresh-token death** **[docs]**: the user revokes access; the
  token is **unused for six months**; the password changes (only for Gmail
  scopes); the account passes **100 live refresh tokens per client ID**, at
  which point the oldest is silently invalidated. A server that re-runs
  consent on every restart would eventually hit that limit.
- **Client type:** a **Desktop app** client uses the installed-app flow with a
  loopback redirect (`http://127.0.0.1:<port>`), PKCE recommended. The client
  secret is optional in the exchange, and the out-of-band (OOB) copy-the-code
  flow is **deprecated and no longer supported**. **[docs]**
- **The LAN problem:** the loopback redirect has to reach a listener on the
  machine whose browser gives consent. On a headless `todo api` host, the
  browser is on another machine. **[docs]** (loopback semantics) Workable
  paths, all **[inferred]**:
  1. Run consent once on the server through an SSH port-forward
     (`ssh -L <port>:127.0.0.1:<port>`), so the laptop's browser redirects
     into the server's listener.
  2. Run consent on a laptop and copy the resulting refresh token to the
     server.
  3. A Web-application client redirecting to the LAN host. Google requires
     web redirect URIs to be HTTPS on a public domain, or `localhost`, so a
     bare LAN IP or `.lan` name is likely rejected. This needs testing.
- **Device flow is not an option:** the limited-input device flow supports
  only `openid/email/profile`, `drive.appdata`/`drive.file` and `youtube*`.
  The Tasks scope is not on the list. **[docs]**
- **Practical result:** a Desktop client, with the consent screen switched to
  **In production** and left unverified, consented once through an SSH tunnel,
  and the refresh token stored on the host with the access token refreshed on
  use. The alternative is staying in Testing and re-consenting every 7 days.
  **[inferred]**
- gtasks-md stores the Desktop `credentials.json` under
  `$XDG_DATA_HOME/gtasks-md/<user>/`, runs
  `InstalledAppFlow.run_local_server(port=0)` on first use, caches `token.json`
  under `$XDG_CACHE_HOME`, and refreshes it when expired. It assumes the
  browser and the CLI share a machine. **[inferred from gtasks-md code]**

---

## 5. What cannot round-trip

Tracker attributes (CONTEXT.md, ADR 0004) against the Google Task:

| Tracker side | Google side | Outcome |
| --- | --- | --- |
| Subtasks to **five levels** | One level **[inferred]** | Depth 2 and deeper cannot be represented. They must be flattened (for example into the top-level Task's subtasks, or into notes) or left unsynced. |
| Subtask **never moves** to another parent | The app lets a person drag a subtask to any parent or to the top level. `move` does the same. **[docs]** | Google can produce a change the tracker forbids. Sync must refuse it, or turn it into delete plus re-create. |
| Completed subtask | After `clear`, a completed and hidden task **cannot be nested** **[docs]** | Cleared subtasks may come back parentless. **[inferred]** |
| **Declined** | Only `needsAction`/`completed` **[docs]** | It collapses to completed, or to a marker in notes. It is lost on the way back unless it is encoded. |
| Doing / deferred / inboxed / backlogged states | No field **[docs]** | Not representable except as text in the title or notes, or as a list per state. |
| **Deleted** | `deleted: true` tombstone **[docs]** | Round-trips, if the tombstone is still retained when polled. **[inferred]** |
| Due **date and time** | Date only; time discarded **[docs]** | The time is lost. |
| **Series** (recurrence) | No field. Repeating tasks cannot be nested or moved between lists **[docs]** | Cannot be written. A Google-side repeat is seen as a single dated task. **[inferred]** |
| **Tags**, **Color** | No fields **[docs]** | Lost, unless encoded in title or notes text. |
| **Attachment** (path or URL) | `links[]` is read-only **[docs]** | Cannot be written as a link. It can only be text in notes. |
| **Actor** (who wrote it) | No author field **[docs]** | A write from Google has no Actor. The sync itself has to be the Actor on the git commit. **[inferred]** |
| Order | `position` is opaque and settable only by `move`/`previous` **[docs]** | Round-trips for visible tasks. Hidden and completed tasks can only sit at position 0 **[docs]**. |
| Title over 1024 or notes over 8192 characters | Hard limits **[docs]** | Must be truncated or refused. |
| Repo → task list | TaskList has only a title **[docs]** | The mapping must be kept on our side, by id rather than by title (see gtasks-md below). |
| Assigned tasks (Docs or Chat) | Cannot be inserted, cannot have notes, cannot be parents **[docs]** | Read-only imports at most. |

### How gtasks-md copes

- **It narrows the model.** It carries only id, title, notes, position, status
  and one level of subtasks. Due dates and links are not mapped at all.
  **[inferred from gtasks-md code]** Because it `patch`es only title, notes and
  status, it leaves a task's `due` untouched on update. **[inferred]**
- **It matches by title** at every level. A rename is therefore a delete plus
  an insert, which loses the id, `due`, `links` and any Google-only state.
  Duplicate titles collapse into one. **[inferred from gtasks-md code]** This
  is the main thing not to copy: match on a stored Google id.
- **It does not detect changes.** Every run takes a full snapshot
  (`showCompleted`, with `showHidden` set to match, and `completedMin`
  defaulting to one week ago), with no `updatedMin` and no `showDeleted`.
  **[inferred from gtasks-md code]**
- **It does not check for conflicts.** It is last-writer-wins against the
  snapshot taken at the start of the command, with a ring of 10 local backups
  and `rollback` as the safety net. **[inferred from gtasks-md code]** ADR 0004
  rules out last-writer-wins for this tracker.
- **It skips reordering completed tasks**, because Google rejects moving hidden
  ones. It reorders with sequential `move` calls. **[inferred from gtasks-md
  code]** This matches the documented position-0 restriction.

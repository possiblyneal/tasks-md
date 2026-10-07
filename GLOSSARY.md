# Tasks

A task tracker with two kinds of consumer: a person at a keyboard, and agents that add, edit, and delete while nobody is watching. The language below exists to keep those two from meaning different things by the same word.

## Contexts

Three boundaries divide the language. A context is an area of the business, not a program and not a folder: all three ship as one artifact, and that is settled rather than coincidental.

**Tracking**:
The area concerned with what work exists and how it is filed. It changes when the way work is organised changes.
_Avoid_: Tasks, core, domain

**Scheduling**:
The area concerned with repetition. It holds rules and the dates they produce and never task content — no title, tag, priority, or attachment lives here. A Series is written on a Task, but what it copies forward is Tracking's.
_Avoid_: Recurrence engine, scheduler, cron

**Change History**:
The area concerned with who did what and in what order, and also the name of the record at its centre. It changes when the arrangement of agents changes, not when the way work is organised does. It keeps writes without reading inside them, so a new Task attribute changes Tracking alone.
_Avoid_: Audit, provenance, logging

## Language

### Tracking

**Task**:
A single piece of work the tracker holds, always in exactly one State. There is one kind of Task: what a person works on and what an agent works on are the same thing, seen two ways.
_Avoid_: Todo, item, entry, ticket

**State**:
Where a Task stands: Inbox, Backlog, Doing, Deferred, Done or Declined. A Task with no Subtasks moves from any State to any other, and a Subtask has the same six. A parent's State is worked out from its Subtasks and never set: while any has not ended, it is the first of Doing, Backlog, Inbox and Deferred that one of them is in; once all have ended, it is Declined if every one was Declined and Done otherwise. So a parent ends only once every Subtask beneath it has ended, and it ends on the day the last of them did. `tasks` writes a parent's State into its line like any other; a line that says otherwise is shown as breaking the rule, and the State worked out is the one that counts. A Task added without a State named is in Inbox, whoever added it. Done and Declined are the two that **end** a Task.
_Avoid_: Status, column, lane, stage

**Inbox**:
The State of a Task nobody has filed yet. It is where anything added lands, by a person, a dump or an Agent, until a person moves it.
_Avoid_: Triage, new, unsorted

**Backlog**:
The State of a Task that is filed and waiting to be started.
_Avoid_: Todo, queue, planned

**Doing**:
The State of a Task somebody is working on. Moving a Task to Doing is how anybody, person or Agent, takes it: a Task already in Doing is taken, and moving it to Doing again is refused rather than shared. It never lapses; a Task left in Doing stays there until somebody moves it.
_Avoid_: In progress, active, started, claimed

**Deferred**:
The State of a Task put aside for now, optionally until a date. When that date passes the Task is moved to Backlog, whatever it was before, and the move is written like any other: by whatever next reads the Tasks, or at midnight if nothing does. A date that passed while nothing was running is caught up then, never skipped. With no date it stays Deferred until somebody moves it. It may carry a Reason.
_Avoid_: Snoozed, hidden, paused, someday

**Done**:
The State of a Task whose work is finished. It ends the Task.
_Avoid_: Completed, closed, checked

**Declined**:
The State of a Task that will not be done. It ends the Task the way Done does. It is not a deletion: a deleted Task is one that should not have been there, and a declined one was there, was looked at, and was refused, optionally with a Reason. Moving it to another State undoes it, which is what makes declining the way to refuse a Task and keep it.
_Avoid_: Rejected, cancelled, dropped, won't-do

**Deleted**:
Said of a Task that should not have been there. It is not a State: it leaves its Repo's file and every read, including the one that shows everything, and only the Repo's history still holds it. A Task somebody may want again is declined rather than deleted.
_Avoid_: Removed, archived, trashed

**Subtask**:
A Task nested under another Task, to five levels. A Subtask is fixed where it was created: it never moves to a different parent and never leaves the Task it sits under. A top-level Task and everything nested beneath it are written and kept correct as one whole.
_Avoid_: Child task, step, checklist item

**Repo**:
A folder holding a TASKS.md, whether or not it holds code, standing for where the work sits: a house move, a job, a codebase. Only a folder directly inside one of the configured roots (by default `~/code`), or a link placed there, is a Repo; a TASKS.md any deeper is not. A Task lives in exactly one Repo. Its TASKS.md belongs to the folder rather than to any branch of the folder's code: switching branches leaves it as it is, and `tasks` run from a linked worktree reads and writes the main checkout's.
_Avoid_: List, project, folder, category, bucket

**Tag**:
A label typed on a Task, standing for what the work is about rather than where it sits. It is text and nothing more: it exists while some Task carries it, it is counted across every Repo by reading the Tasks, and renaming one rewrites every Task that carries it. It has no Color.
_Avoid_: Keyword, label, topic, Collection

**Color**:
One of nine offered colors a Task or a Repo can carry: red, orange, yellow, green, cyan, blue, violet, magenta, brown. It is named rather than coded, and it is one of the nine or it is none — nothing else is stored, so every surface knows how to paint whatever it reads back.
_Avoid_: Colour, hex, swatch, highlight, theme

**Estimate**:
How big a Task's work looks: small, medium or large, in that order. It is a size rather than a duration, and it is the only thing the list sorts by when ordered by estimate.
_Avoid_: Duration, time estimate, points

**Reason**:
Why a Task was deferred or declined, in somebody's own words. It is optional, it belongs to the Task only while the Task is Deferred or Declined, and moving the Task to any other State clears it.
_Avoid_: Blocked reason, note, why

**Stale write**:
A write made against a copy of a top-level Task, and everything nested under it, that has changed since the writer read it. It is refused, and the writer reads again; a write that touches only other Tasks in the same Repo is not stale. Nothing is held before writing, and a person's write and an Agent's are refused alike.
_Avoid_: Conflict, Lease, lock, claim

**Attachment**:
A pointer held on a Task to something living outside the tracker — a file path or a web address. The tracker never holds a copy, so it cannot tell whether what is pointed at still exists.
_Avoid_: File, upload, document

**Narrowing**:
What one read of the Tasks asks for: which Repo, which States, which Tags, what text to match, whether Blocked Tasks are in, and what order to come back in. It describes a question and never a result, nothing stores one, and every Task that comes back came back because the store answered it — a surface narrows by asking for a narrower list rather than by keeping a filter of its own over the one it has. Naming a second Tag widens it rather than narrowing twice: a Task carrying any one of the named Tags is in the read.
_Avoid_: Filter, query, view, search

**Overdue**:
A condition true of a Task whose due date has passed, evaluated whenever something reads it. Nothing records the moment it becomes true.
_Avoid_: Late, expired

**Blocked**:
A condition true of a Task while any Task it names as blocking it, in the same Repo, has not ended. Done and Declined both end a blocker. Like Overdue it is evaluated whenever something reads it and is not a State.
_Avoid_: Waiting, dependent, stuck

### Scheduling

**Series**:
A recurrence rule written on a Task, carried forward from one Occurrence to the next. Editing the rule on the current Occurrence edits the Series; deleting it ends the Series and leaves an ordinary Task.
_Avoid_: Repeat, schedule, template, recurring task, Detached

**Occurrence**:
A Task that carries a Series. Only the current one exists: when it ends, Done or Declined, and by whatever route, the next is written as a new Task in Backlog, and the rule moves to it off the ended one, with its Subtasks reset to Backlog, and with the first date the rule produces after both today and the ended one's date as its Deadline. So an ended Task still carrying a rule is one whose next has not been written yet. A skipped date is a Declined Occurrence, and the Series goes on.
_Avoid_: Instance, event, repetition

### Change History

**Change History**:
The git history of each Repo's TASKS.md, kept apart from the Repo's code history and pushed after every write, one entry per write, each naming its Actor. It is kept rather than collapsed, and it is a record of the file rather than the store: the file is what counts, and the history says how it got that way. Only `tasks` writes the file; it is there to be read. A change that is made by hand anyway, outside `tasks`, enters the history as a direct edit whose Actor is unknown, and nothing tries to name one.
_Avoid_: Log, audit log, journal, event stream

**Actor**:
Whoever performed a write: a person, an Agent, or the task server. Every write is attributed; reads are not. Only the Change History knows an Actor as an identity, and nothing else tells Actors apart by kind.

An Agent names itself `<harness>/<model>` — `claude-code/claude-opus-5`, `orca/fable-5-1` — and a person is the name they have set for themselves, so inside the Change History the slash is what says which wrote a thing and the two halves are what a history log reads. Nowhere else looks at the slash: an Actor is still opaque everywhere the entries are not. The model half is whatever served the call and is not a list anything here holds: OmniRoute fronts many providers and the set turns over. Nothing validates the shape. An Actor is a string the store writes down and reads back, one that names itself badly or not at all is still an Actor, and the day something refuses a write over its own name is the day attribution has started deciding what may be written. The task server is the Actor for writes nobody asked for: a Deferred Task waking to Backlog, or the next Occurrence of a Series. It is the Actor whichever surface's read happened to bring them about.
_Avoid_: User, author, owner

**Agent**:
An actor that is invoked, acts, and exits. It has no schedule of its own and is not running between invocations. It finds work by reading the Tasks narrowed like anybody else, takes one by moving it to Doing, and files what it discovers along the way into Inbox; nothing is chosen for it. It is named for what it does here, which is write to the tracker under attribution, and not for whether it infers. The Broker infers and is not an Agent.
_Avoid_: Bot, worker, daemon, service

### Outside the contexts

**Broker**:
The thing that infers, reached over the network and owned by none of the three contexts. It is shown Tasks and answers with questions, proposals, or one Task read out of a dump; it holds nothing between calls, is never an Actor, and never writes. A proposal becomes a Task only when a person approves it, and the write is attributed to that person. A Task read out of a dump is not a proposal: it is what somebody already said they wanted, so running `tasks capture` over their own words is the approval, and the write is attributed to them the same way.
_Avoid_: The box, the agent, the AI, the model, the assistant

**Dump**:
What somebody says a Task is, in their own words and in one box, before any field is filled in. The Broker reads one and answers with the Task it describes, so what comes back is that person's own words sorted into attributes rather than a suggestion of work nobody asked for. A dump amending a Task carries that Task as it stands and comes back whole. The Broker's answer names a Repo, which a person approves on the form; `tasks capture` writes to the Repo its caller names and never to the Broker's guess.
_Avoid_: Prompt, note, request

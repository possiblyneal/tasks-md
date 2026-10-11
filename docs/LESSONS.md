# Lessons learned

A lesson is a durable, repeatable, non-obvious constraint, convention, or tool quirk; leave out incident history and generic advice.
Each lesson has a short imperative `##` title, the rule, **Do:** the required action, **Why:** the consequence (only when not self-evident), and **Source:** a link to the authority.

## Lessons

## Never start a commit body line with a word and a colon

commitlint reads a line beginning `word:` as a footer token, so a body
paragraph that begins one is a footer without a blank line above it and the
commit is refused with `footer-leading-blank`. The message on disk is fine; it
is the parse that disagrees.

**Do:** Reword the line so the colon is not at the start of it, or put the
colon after the first clause.

**Why:** The reported rule names the footer, and the body line it is really
about is not quoted, so the obvious reading is that the trailers are wrong when
nothing about them is.

**Source:** [.commitlintrc.yaml](../.commitlintrc.yaml)

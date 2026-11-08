# Commit rules

This repository follows [Scoped Commits](https://scopedcommits.com/).
A subject has one form: `<scope>: <description>`.

## Subject

- `scope` names the subsystem, area, or module. Use the package, folder, or feature name, such as `lint`, `semantic`, `renderer`, `lsp`, `cli`, or `docs`.
- Put the scope first. Do not put a type before it. A commit is not `fix(lint): …` or `feat: …`; it is `lint: …`.
- Use one scope per commit. If the commit covers several areas, use the broader scope that contains them, or list the scopes with a comma.
- Use `treewide` when the commit changes the whole tree.
- `description` states the change in one short sentence. Start with a lowercase word. Do not end with a period.
- Keep the description imperative and concrete. Name the result, not the activity.

## Body and trailers

- Add a body only when the subject cannot carry the reason. Separate it from the subject with one blank line.
- Put metadata in trailers, one per line. Use `Refs:` or `Jira-Ticket:` and the identifier.
- A ticket number can also follow the scope in parentheses: `auth (PROJ-123): fix the login bug`.
- A merge commit and a revert commit keep the format that Git generates.

## Limits

- Do not generate a changelog from the commit log. A commit log serves contributors; a changelog serves users.
- Do not edit an old subject to change its meaning. Rewrite it only to fit this format.

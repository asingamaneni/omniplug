---
name: summarize-changes
description: "Summarize uncommitted changes and flag risks. Use when asked: what changed?"
whenToUse: When the user asks what changed or wants a commit message.
allowedTools:
  - Read
  - Grep
model: balanced
---

Summarize the current changes in two or three bullet points, then list any
risks you notice such as missing error handling or hardcoded values.

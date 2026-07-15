---
name: "summarize-changes"
description: "Summarize uncommitted changes and flag risks. Use when asked: what changed?"
when_to_use: "When the user asks what changed or wants a commit message."
allowed-tools: ["Read", "Grep"]
model: sonnet
---

Summarize the current changes in two or three bullet points, then list any
risks you notice such as missing error handling or hardcoded values.

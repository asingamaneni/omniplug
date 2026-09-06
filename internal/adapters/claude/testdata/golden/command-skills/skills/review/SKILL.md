---
name: "review"
description: "Review the current change."
argument-hint: "[range]"
allowed-tools: ["Read", "Grep"]
model: sonnet
disable-model-invocation: true
user-invocable: true
"x-claude-only": "kept"
---

Review $ARGUMENTS.

---
description: "Deploy the application to production."
argument-hint: "[environment]"
allowed-tools: ["Bash(git push *)"]
disable-model-invocation: true
---

Deploy $ARGUMENTS to production:

1. Run the test suite.
2. Build the application.
3. Push to the deployment target.

<!--
Copyright (c) 2025 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->
# Description
A few sentences describing the overall goals of the pull request's commits.

# JIRA Issues
List the JIRA issues impacted by this PR:

| JIRA Issue # |
| -------------- |
| |

# Checklist:

- [ ] I have performed a self-review of my own code to ensure there are no formatting, vetting, linting, or security issues
- [ ] I have verified that new and existing unit tests pass locally with my changes
- [ ] I have not allowed coverage numbers to degenerate
- [ ] I have maintained at least 90% code coverage
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] I have verified that all changes maintain idempotent behavior and can be safely re-executed without causing unintended side effects.
- [ ] Backward compatibility is not broken

# How Has This Been Tested?
Please describe the tests that you ran to verify your changes. Please also list any relevant details for your test configuration

- [ ] E2E tests were run
  - [ ] **PowerFlex** (`./run-e2e-test.sh --pflex`)
  - [ ] **PowerScale** (`./run-e2e-test.sh --pscale`)
  - [ ] **PowerStore** (`./run-e2e-test.sh --pstore`)
  - [ ] **PowerMax** (`./run-e2e-test.sh --pmax`)
  - [ ] **Unity** (`./run-e2e-test.sh --unity`)
  - [ ] **COSI** (`./run-e2e-test.sh --cosi`)
  - [ ] **Sanity** (`./run-e2e-test.sh --sanity`)
  - [ ] **No modules** (`./run-e2e-test.sh --no-modules`)
  - [ ] **Authorization** (`./run-e2e-test.sh --auth`)
  - [ ] **Authorization Proxy Server** (`./run-e2e-test.sh --auth-proxy`)
  - [ ] **Replication** (`./run-e2e-test.sh --replication`)
  - [ ] **Observability** (`./run-e2e-test.sh --obs`)
  - [ ] **Resiliency** (`./run-e2e-test.sh --resiliency`)
  - [ ] **Zoning (PowerFlex zoning)** (`./run-e2e-test.sh --zoning`)
- [ ] Test A
- [ ] Test B

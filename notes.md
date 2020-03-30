# cri-tools v1.18.0

cri-tools v1.18.0 mainly focuses on bug fixes and stability improvements.

# Main Changes

- #559 Switch to urfave/cli/v2

# CRI CLI (crictl)

- #575 Add go-template option for inspect commands
- #570 Fix invalid `log_path` in docs

# CRI validation testing (critest)

- #576 Make apparmor failure test more flexible
- #574 Start container before fetching metrics
- #567 Cleanup container create test to reduce duplication
- #566 Add container stats test

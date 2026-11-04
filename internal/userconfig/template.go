package userconfig

// starterConfig is written the first time caged needs the config file. Every
// key is commented out, so the file parses as an empty config and the defaults
// stay in force until the user edits it. Lines that start with "## " are
// prose; lines that start with "# " are examples, valid YAML once the "# " is
// removed.
const starterConfig = `## caged configuration. Every example below is commented out, so caged
## behaves as if this file were empty. Remove the leading "# " to enable one.
##
## This folder is mounted read/write at /home/caged/.config/caged inside the
## container, so an agent can edit this file. Caged shows you every change as
## a diff and asks for approval before the next run uses it.

# version: 1

## Extra apt packages added to the runtime image (built as caged-runtime:ext).
# image:
#   apt:
#     - jq
#     - ripgrep

## Agent config folders may contain symlinks (for example Stow-managed
## dotfiles). A link is followed only when its target is under links.allow.
## read_only mounts the target read-only. links.deny extends the built-in list.
# links:
#   allow:
#     - path: ~/dotfiles
#       read_only: true
#   deny:
#     - ~/secrets

## Limit which project folders caged may mount. When this key is present only
## folders inside these roots can be mounted, even with --allow-broad-mount.
# workdirs:
#   allow:
#     - path: ~/git
`

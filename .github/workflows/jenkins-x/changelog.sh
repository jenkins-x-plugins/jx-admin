#!/usr/bin/env sh

git config --global --add safe.directory /github/workspace
jx changelog create --verbose --output-markdown=changelog.md


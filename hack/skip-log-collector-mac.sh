#!/usr/bin/env bash

find . -name .goreleaser.yml -exec sed -i '' '/binary: log-collector/{
a\
  skip: true
}' {} +

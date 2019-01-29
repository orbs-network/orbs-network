#!/bin/bash -x

source ./test.common.sh

go test -tags "cpunoise norecover" ./test/acceptance -count 100 -timeout 20m -failfast > _out/test.out
check_exit_code_and_report

go test -tags "cpunoise norecover" ./services/blockstorage/test -count 100 -timeout 7m -failfast > _out/test.out
check_exit_code_and_report

go test -tags "cpunoise norecover" ./services/blockstorage/internodesync -count 100 -timeout 7m -failfast > _out/test.out
check_exit_code_and_report

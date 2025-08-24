#!/bin/bash

RESULT=$(docker run --rm --network=tp0_testing_net busybox -c "echo hello | nc server 12345")

if [ "$RESULT" = "hello" ]; then
  echo "action: test_echo_server | result: success"
else
  echo "action: test_echo_server | result: fail"
fi
#!/bin/bash
echo "=== Gruntdeck Demo Script ==="
echo "Working directory: $(pwd)"
echo "Arguments received: $@"
echo "Arg 1: $1"
echo "Arg 2: $2"
echo "Reading transferred config file:"
cat /tmp/gruntdeck_test/config_demo.txt
echo "============================="

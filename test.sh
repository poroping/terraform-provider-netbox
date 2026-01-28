#!/bin/bash
# Test runner script for NetBox provider acceptance tests
# This script provides examples for running different types of tests

set -e

echo "NetBox Terraform Provider Test Runner"
echo "======================================"
echo ""

# Check for required environment variables
if [ -z "$NETBOX_URL" ]; then
    echo "Error: NETBOX_URL environment variable is not set"
    echo "Example: export NETBOX_URL=https://demo.netbox.dev"
    exit 1
fi

if [ -z "$NETBOX_TOKEN" ]; then
    echo "Error: NETBOX_TOKEN environment variable is not set"
    echo "Example: export NETBOX_TOKEN=your-api-token-here"
    exit 1
fi

echo "Configuration:"
echo "  NetBox URL: $NETBOX_URL"
echo "  Token: [REDACTED]"
echo ""

# Parse command line arguments
TEST_TYPE="${1:-all}"

case $TEST_TYPE in
    unit)
        echo "Running unit tests..."
        go test -v -cover -timeout=120s -parallel=4 ./internal/...
        ;;
    
    acceptance)
        echo "Running acceptance tests..."
        echo "This will create/modify/delete resources in NetBox at: $NETBOX_URL"
        read -p "Continue? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            TF_ACC=1 go test ./internal/provider/... -v -timeout 120m
        else
            echo "Aborted."
            exit 1
        fi
        ;;
    
    resource)
        if [ -z "$2" ]; then
            echo "Error: Please specify a resource name"
            echo "Example: ./test.sh resource tenant"
            exit 1
        fi
        echo "Running acceptance tests for resource: $2"
        TF_ACC=1 go test ./internal/provider/... -v -run="TestAcc.*${2^}Resource" -timeout 120m
        ;;
    
    datasource)
        if [ -z "$2" ]; then
            echo "Error: Please specify a data source name"
            echo "Example: ./test.sh datasource tenant"
            exit 1
        fi
        echo "Running acceptance tests for data source: $2"
        TF_ACC=1 go test ./internal/provider/... -v -run="TestAcc.*${2^}DataSource" -timeout 120m
        ;;
    
    validators)
        echo "Running validator tests..."
        go test -v ./internal/validators/... -timeout 30s
        ;;
    
    all)
        echo "Running all tests..."
        echo "1. Unit tests"
        go test -v -cover -timeout=120s -parallel=4 ./internal/...
        echo ""
        echo "2. Acceptance tests (requires NetBox instance)"
        read -p "Run acceptance tests? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            TF_ACC=1 go test ./internal/provider/... -v -timeout 120m
        else
            echo "Skipping acceptance tests."
        fi
        ;;
    
    *)
        echo "Unknown test type: $TEST_TYPE"
        echo ""
        echo "Usage: $0 [test-type] [resource-name]"
        echo ""
        echo "Test types:"
        echo "  unit        - Run unit tests only"
        echo "  acceptance  - Run all acceptance tests (requires NetBox)"
        echo "  resource    - Run acceptance tests for specific resource"
        echo "  datasource  - Run acceptance tests for specific data source"
        echo "  validators  - Run validator tests"
        echo "  all         - Run all tests (default)"
        echo ""
        echo "Examples:"
        echo "  $0 unit"
        echo "  $0 acceptance"
        echo "  $0 resource tenant"
        echo "  $0 datasource vrf"
        exit 1
        ;;
esac

echo ""
echo "Tests completed successfully!"
